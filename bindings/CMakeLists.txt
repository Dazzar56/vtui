cmake_minimum_required(VERSION 3.14)
project(vtui_bindings C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_CXX_STANDARD 17)

# 1. Build Go shared library libvtui
set(GO_CABI_DIR "${CMAKE_CURRENT_SOURCE_DIR}/c/cabi")
set(VTUI_LIB_DIR "${CMAKE_CURRENT_BINARY_DIR}/lib")
file(MAKE_DIRECTORY ${VTUI_LIB_DIR})

if(WIN32)
    set(VTUI_LIB_NAME "vtui.dll")
    set(VTUI_LIB_PATH "${VTUI_LIB_DIR}/${VTUI_LIB_NAME}")
elseif(APPLE)
    set(VTUI_LIB_NAME "libvtui.dylib")
    set(VTUI_LIB_PATH "${VTUI_LIB_DIR}/${VTUI_LIB_NAME}")
else()
    set(VTUI_LIB_NAME "libvtui.so")
    set(VTUI_LIB_PATH "${VTUI_LIB_DIR}/${VTUI_LIB_NAME}")
endif()

add_custom_command(
    OUTPUT ${VTUI_LIB_PATH}
    COMMAND go build -buildmode=c-shared -o ${VTUI_LIB_PATH} ${GO_CABI_DIR}
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/..
    COMMENT "Building Go C-shared library: ${VTUI_LIB_NAME}"
)

add_custom_target(vtui_shared ALL DEPENDS ${VTUI_LIB_PATH})

add_library(vtui_c_import SHARED IMPORTED GLOBAL)
set_target_properties(vtui_c_import PROPERTIES
    IMPORTED_LOCATION "${VTUI_LIB_PATH}"
)
add_dependencies(vtui_c_import vtui_shared)

add_subdirectory(c)
add_subdirectory(cpp)
