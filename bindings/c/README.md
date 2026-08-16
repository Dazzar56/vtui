# C Bindings for vtui

This directory provides native C language bindings and an immediate-mode facade for `vtui`.

## Prerequisites

- Go compiler (1.26+)
- CMake (3.14+)
- C compiler (GCC, Clang, or MSVC)

## Building with CMake

```bash
mkdir -p build && cd build
cmake ../bindings
cmake --build .
```

The build produces:
- `lib/libvtui.so` (or `.dylib` / `.dll` on macOS/Windows)
- `c/hello_c` (C Hello vtui demo executable)
- `cpp/hello_cpp` (C++ Hello vtui demo executable)

## Building manually with GCC / Clang

```bash
# 1. Compile the shared library from Go
go build -buildmode=c-shared -o bindings/c/lib/libvtui.so ./bindings/c/cabi

# 2. Compile the C application
gcc -Ibindings/c/include bindings/c/src/vtui.c bindings/c/examples/hello.c \
    -Lbindings/c/lib -lvtui -Wl,-rpath,bindings/c/lib -o hello_c

# 3. Run
./hello_c
```
