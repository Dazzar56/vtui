add_library(vtui_c STATIC
    src/vtui.c
)

target_include_directories(vtui_c PUBLIC
    include
)

target_link_libraries(vtui_c PUBLIC
    vtui_c_import
)

add_executable(hello_c
    examples/hello.c
)

target_link_libraries(hello_c PRIVATE
    vtui_c
)
