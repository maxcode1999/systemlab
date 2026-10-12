cmake_minimum_required(VERSION 3.15)
project(liba CXX)

find_package(libb REQUIRED CONFIG)

add_library(a src/a.cpp)
target_include_directories(a PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)
set_target_properties(a PROPERTIES PUBLIC_HEADER include/a.h)
target_link_libraries(a PUBLIC libb::libb)

install(TARGETS a)
install(FILES include/a.h DESTINATION include)
