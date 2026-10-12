cmake_minimum_required(VERSION 3.15)
project(libb CXX)

add_library(b src/b.cpp)
target_include_directories(b PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)
set_target_properties(b PROPERTIES PUBLIC_HEADER include/b.h)

install(TARGETS b)
install(FILES include/b.h DESTINATION include)
