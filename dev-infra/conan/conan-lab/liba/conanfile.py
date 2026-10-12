# Middle of the chain: liba depends on libb; app depends on liba.
from conan import ConanFile
from conan.tools.cmake import CMake, cmake_layout


class LibA(ConanFile):
    name = "liba"
    version = "1.0"

    settings = "os", "compiler", "build_type", "arch"

    exports_sources = "src/*", "include/*", "CMakeLists.txt"

    generators = "CMakeToolchain", "CMakeDeps"

    def requirements(self):
        # libb version normally affects liba's package_id (requires-mode).
        self.requires("libb/1.0")  # use libb/2.0 when running the bump experiment

    def layout(self):
        cmake_layout(self)

    def build(self):
        cmake = CMake(self)
        cmake.configure()
        cmake.build()

    def package(self):
        cmake = CMake(self)
        cmake.install()

    def package_info(self):
        self.cpp_info.libs = ["a"]

    # --- Optional lab experiment (requires-mode) ---
    # Default: liba's package_id changes when libb's version changes.
    # Uncomment unrelated_mode() only if liba does not expose libb in its public ABI.
    #
    # def package_id(self):
    #     self.info.requires["libb"].unrelated_mode()
