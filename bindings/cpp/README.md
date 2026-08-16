# C++ Bindings for vtui

Modern, header-only C++17 wrapper for `vtui`.

## Usage Example

```cpp
#include <vtui.hpp>

int main() {
    return vtui::run([](vtui::Ui& u) {
        auto d = u.dialog(" Hello vtui ", {.w = 40});
        auto name = u.edit("&Name:", "Type here...");
        if (u.button("&Ok")) {
            u.message(" Result ", "You typed:\n" + name);
        }
    });
}
```

## Building with CMake

```bash
mkdir -p build && cd build
cmake ../bindings
cmake --build .
./cpp/hello_cpp
```
