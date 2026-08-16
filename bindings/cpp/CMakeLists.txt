add_executable(hello_cpp
    examples/hello.cpp
)

target_include_directories(hello_cpp PRIVATE
    include
    ../c/include
)

target_link_libraries(hello_cpp PRIVATE
    vtui_c_import
)
