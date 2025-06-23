#include <cuda_runtime.h>
#include <device_launch_parameters.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <algorithm>
#include <vector>
#include <string>
#include <fstream>
#include <iostream>
#include <chrono>

#define WORD_LENGTH 5
#define MAX_WORDS 15000
#define CUDA_CHECK(call) \
    do { \
        cudaError_t err = call; \
        if (err != cudaSuccess) { \
            fprintf(stderr, "CUDA error at %s:%d - %s\n", __FILE__, __LINE__, cudaGetErrorString(err)); \
            exit(1); \
        } \
    } while(0)

// Device function to calculate hint for a guess-answer pair
__device__ unsigned char getHint(const char* guess, const char* answer) {
    unsigned char charHints[5] = {0};
    
    // Check for exact matches first
    for (int i = 0; i < 5; i++) {
        if (guess[i] == answer[i]) {
            charHints[i] = 2; // Green (correct position)
        }
    }
    
    // Check for wrong position matches
    for (int i = 0; i < 5; i++) {
        if (charHints[i] == 0) { // Not already green
            for (int j = 0; j < 5; j++) {
                if (i != j && charHints[j] != 2 && guess[i] == answer[j]) {
                    // Make sure this letter isn't already accounted for
                    bool alreadyUsed = false;
                    for (int k = 0; k < i; k++) {
                        if (charHints[k] == 1 && guess[k] == guess[i]) {
                            alreadyUsed = true;
                            break;
                        }
                    }
                    if (!alreadyUsed) {
                        charHints[i] = 1; // Yellow (wrong position)
                        break;
                    }
                }
            }
        }
    }
    
    // Convert to single hint value (base 3)
    unsigned char ret = 0;
    for (int i = 0; i < 5; i++) {
        ret = (ret * 3) + charHints[i];
    }
    
    return ret;
}

// Kernel to calculate all hints for all guess-answer pairs
__global__ void calculateAllHints(char* guesses, char* answers, unsigned char* hints, 
                                  int numGuesses, int numAnswers) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    int total = numGuesses * numAnswers;
    
    if (idx < total) {
        int guessIdx = idx / numAnswers;
        int answerIdx = idx % numAnswers;
        
        char* guess = guesses + guessIdx * (WORD_LENGTH + 1);
        char* answer = answers + answerIdx * (WORD_LENGTH + 1);
        
        hints[idx] = getHint(guess, answer);
    }
}

// Device function to count matching answers for a hint
__device__ int countMatchingAnswers(char* guesses, char* answers, unsigned char* hints,
                                   int guessIdx, unsigned char targetHint, int numAnswers) {
    int count = 0;
    for (int i = 0; i < numAnswers; i++) {
        int hintIdx = guessIdx * numAnswers + i;
        if (hints[hintIdx] == targetHint) {
            count++;
        }
    }
    return count;
}

// Device function to calculate average number of candidates for a guess pair
__device__ float avgNumCandidates(char* guesses, char* answers, unsigned char* hints,
                                 int guess1Idx, int guess2Idx, int numAnswers) {
    float total = 0.0f;
    
    for (int answerIdx = 0; answerIdx < numAnswers; answerIdx++) {
        // Get hint for first guess against this answer
        unsigned char hint1 = hints[guess1Idx * numAnswers + answerIdx];
        
        // Count how many answers match this hint for first guess
        int candidates = countMatchingAnswers(guesses, answers, hints, guess1Idx, hint1, numAnswers);
        
        if (candidates <= 2) {
            total += 1.0f;
        } else {
            // Apply second guess to remaining candidates
            int finalCandidates = 0;
            for (int i = 0; i < numAnswers; i++) {
                if (hints[guess1Idx * numAnswers + i] == hint1) {
                    unsigned char hint2 = hints[guess2Idx * numAnswers + i];
                    // Count how many of the remaining candidates match hint2
                    for (int j = 0; j < numAnswers; j++) {
                        if (hints[guess1Idx * numAnswers + j] == hint1 && 
                            hints[guess2Idx * numAnswers + j] == hint2) {
                            finalCandidates++;
                        }
                    }
                    break; // We only need to do this calculation once per unique hint1
                }
            }
            total += (float)finalCandidates;
        }
    }
    
    return total / (float)numAnswers;
}

// Kernel to find best guess pairs
__global__ void findBestGuessPairs(char* guesses, char* answers, unsigned char* hints,
                                  int* filteredIndices, float* results, int numFiltered, int numAnswers) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    int totalPairs = numFiltered * (numFiltered - 1) / 2;
    
    if (idx < totalPairs) {
        // Convert linear index to i,j pair indices
        int i = 0;
        int remaining = idx;
        while (remaining >= (numFiltered - 1 - i)) {
            remaining -= (numFiltered - 1 - i);
            i++;
        }
        int j = i + 1 + remaining;
        
        int guess1Idx = filteredIndices[i];
        int guess2Idx = filteredIndices[j];
        
        // Check if guesses share letters (skip if they do)
        char* guess1 = guesses + guess1Idx * (WORD_LENGTH + 1);
        char* guess2 = guesses + guess2Idx * (WORD_LENGTH + 1);
        
        bool shareLetters = false;
        for (int a = 0; a < 5; a++) {
            for (int b = 0; b < 5; b++) {
                if (guess1[a] == guess2[b]) {
                    shareLetters = true;
                    break;
                }
            }
            if (shareLetters) break;
        }
        
        if (!shareLetters) {
            results[idx] = avgNumCandidates(guesses, answers, hints, guess1Idx, guess2Idx, numAnswers);
        } else {
            results[idx] = 999999.0f; // Large value to indicate invalid pair
        }
    }
}

// Host function to load words from file
std::vector<std::string> loadWords(const char* filename) {
    std::vector<std::string> words;
    std::ifstream file(filename);
    std::string line;
    
    while (std::getline(file, line)) {
        if (line.length() == WORD_LENGTH) {
            words.push_back(line);
        }
    }
    
    return words;
}

// Host function to check if word has 5 unique letters
bool hasUniqueLetters(const std::string& word) {
    bool seen[26] = {false};
    for (char c : word) {
        int idx = c - 'a';
        if (seen[idx]) return false;
        seen[idx] = true;
    }
    return true;
}

int main() {
    auto start = std::chrono::high_resolution_clock::now();
    
    // Load word lists
    std::vector<std::string> guessesVec = loadWords("io/guesses.txt");
    std::vector<std::string> answersVec = loadWords("io/answers.txt");
    
    int numGuesses = guessesVec.size();
    int numAnswers = answersVec.size();
    
    printf("Loaded %d guesses and %d answers\n", numGuesses, numAnswers);
    
    // Allocate host memory
    char* h_guesses = (char*)malloc(numGuesses * (WORD_LENGTH + 1) * sizeof(char));
    char* h_answers = (char*)malloc(numAnswers * (WORD_LENGTH + 1) * sizeof(char));
    
    // Copy words to host arrays
    for (int i = 0; i < numGuesses; i++) {
        strcpy(h_guesses + i * (WORD_LENGTH + 1), guessesVec[i].c_str());
    }
    for (int i = 0; i < numAnswers; i++) {
        strcpy(h_answers + i * (WORD_LENGTH + 1), answersVec[i].c_str());
    }
    
    // Allocate device memory
    char* d_guesses;
    char* d_answers;
    unsigned char* d_hints;
    
    CUDA_CHECK(cudaMalloc(&d_guesses, numGuesses * (WORD_LENGTH + 1) * sizeof(char)));
    CUDA_CHECK(cudaMalloc(&d_answers, numAnswers * (WORD_LENGTH + 1) * sizeof(char)));
    CUDA_CHECK(cudaMalloc(&d_hints, numGuesses * numAnswers * sizeof(unsigned char)));
    
    // Copy data to device
    CUDA_CHECK(cudaMemcpy(d_guesses, h_guesses, numGuesses * (WORD_LENGTH + 1) * sizeof(char), cudaMemcpyHostToDevice));
    CUDA_CHECK(cudaMemcpy(d_answers, h_answers, numAnswers * (WORD_LENGTH + 1) * sizeof(char), cudaMemcpyHostToDevice));
    
    // Calculate hints on GPU
    printf("Calculating hints on GPU...\n");
    int totalHints = numGuesses * numAnswers;
    int blockSize = 256;
    int gridSize = (totalHints + blockSize - 1) / blockSize;
    
    calculateAllHints<<<gridSize, blockSize>>>(d_guesses, d_answers, d_hints, numGuesses, numAnswers);
    CUDA_CHECK(cudaDeviceSynchronize());
    
    // Filter guesses with 5 unique letters
    std::vector<int> filteredIndices;
    for (int i = 0; i < numGuesses; i++) {
        if (hasUniqueLetters(guessesVec[i])) {
            filteredIndices.push_back(i);
        }
    }
    
    int numFiltered = filteredIndices.size();
    int totalPairs = numFiltered * (numFiltered - 1) / 2;
    
    printf("Filtered to %d guesses with unique letters (%d pairs)\n", numFiltered, totalPairs);
    
    // Allocate memory for filtered indices and results
    int* d_filteredIndices;
    float* d_results;
    
    CUDA_CHECK(cudaMalloc(&d_filteredIndices, numFiltered * sizeof(int)));
    CUDA_CHECK(cudaMalloc(&d_results, totalPairs * sizeof(float)));
    
    CUDA_CHECK(cudaMemcpy(d_filteredIndices, filteredIndices.data(), numFiltered * sizeof(int), cudaMemcpyHostToDevice));
    
    // Find best guess pairs on GPU
    printf("Finding best guess pairs on GPU...\n");
    gridSize = (totalPairs + blockSize - 1) / blockSize;
    
    findBestGuessPairs<<<gridSize, blockSize>>>(d_guesses, d_answers, d_hints, d_filteredIndices, d_results, numFiltered, numAnswers);
    CUDA_CHECK(cudaDeviceSynchronize());
    
    // Copy results back and find minimum
    float* h_results = (float*)malloc(totalPairs * sizeof(float));
    CUDA_CHECK(cudaMemcpy(h_results, d_results, totalPairs * sizeof(float), cudaMemcpyDeviceToHost));
    
    float bestScore = 999999.0f;
    int bestIdx = -1;
    
    for (int i = 0; i < totalPairs; i++) {
        if (h_results[i] < bestScore) {
            bestScore = h_results[i];
            bestIdx = i;
        }
    }
    
    // Convert best index back to guess pair
    int i = 0, remaining = bestIdx;
    while (remaining >= (numFiltered - 1 - i)) {
        remaining -= (numFiltered - 1 - i);
        i++;
    }
    int j = i + 1 + remaining;
    
    auto end = std::chrono::high_resolution_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::milliseconds>(end - start);
    
    printf("\nBest guess pair: %s, %s (score: %.2f)\n", 
           guessesVec[filteredIndices[i]].c_str(), 
           guessesVec[filteredIndices[j]].c_str(), 
           bestScore);
    printf("Total execution time: %ld ms\n", duration.count());
    
    // Cleanup
    free(h_guesses);
    free(h_answers);
    free(h_results);
    cudaFree(d_guesses);
    cudaFree(d_answers);
    cudaFree(d_hints);
    cudaFree(d_filteredIndices);
    cudaFree(d_results);
    
    return 0;
} 