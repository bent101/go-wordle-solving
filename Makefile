NVCC = nvcc
CFLAGS = -O3 -std=c++17
TARGET = wordle_solver
SOURCE = wordle_solver.cu

# CUDA compute capability (adjust based on your GPU)
COMPUTE_CAPABILITY = -arch=sm_75

all: $(TARGET)

$(TARGET): $(SOURCE)
	$(NVCC) $(CFLAGS) $(COMPUTE_CAPABILITY) -o $(TARGET) $(SOURCE)

clean:
	rm -f $(TARGET)

run: $(TARGET)
	./$(TARGET)

.PHONY: all clean run 