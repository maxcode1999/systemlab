cmake_minimum_required(VERSION 3.15)
project(app CXX)

find_package(liba REQUIRED CONFIG)

add_executable(app src/main.cpp)
target_link_libraries(app PRIVATE liba::liba)
