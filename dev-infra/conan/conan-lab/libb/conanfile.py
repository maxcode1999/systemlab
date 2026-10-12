# Bottom of the chain: shared library libb (no Conan deps).
from conan import ConanFile
from conan.tools.cmake import CMake, cmake_layout


class LibB(ConanFile):
    name = "libb"
    version = "1.0"  # bump to "2.0" and re-export for lockfile / package_id lab steps

    settings = "os", "compiler", "build_type", "arch"

    exports_sources = "src/*", "include/*", "CMakeLists.txt"

    generators = "CMakeToolchain", "CMakeDeps"

    def layout(self):
        cmake_layout(self)

    def build(self):
        cmake = CMake(self)
        cmake.configure()
        cmake.build()

    def package(self):
        # Install headers/libs into the package folder (becomes the Conan package artifact).
        cmake = CMake(self)
        cmake.install()

    def package_info(self):
        # Tells consumers how to link: -lb and CMake target libb::libb via CMakeDeps.
        self.cpp_info.libs = ["b"]
