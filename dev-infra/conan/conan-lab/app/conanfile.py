# Top of the dependency chain: app -> liba -> libb.
# Used for lockfiles, `conan create app`, and cross-glibc binary tests.
from conan import ConanFile
from conan.tools.cmake import CMake, cmake_layout


class AppConan(ConanFile):
    # Recipe identity in the Conan graph (name/version@user/channel if you add them).
    name = "app"
    version = "1.0"

    # Dimensions that affect binary compatibility (hashed into package_id).
    # os: Linux, Macos, ...  compiler: gcc, clang, ...  build_type: Debug/Release  arch: x86_64, ...
    settings = "os", "compiler", "build_type", "arch"

    # Files copied into the Conan cache when you `conan export` / `conan create`.
    exports_sources = "CMakeLists.txt", "src/*"

    # Emit CMake toolchain + find_package() config for dependencies before build().
    generators = "CMakeToolchain", "CMakeDeps"

    def requirements(self):
        # Direct dependency; Conan resolves liba and (transitively) libb.
        self.requires("liba/1.0")

    def layout(self):
        # Standard out-of-source folders: build/, generators/, etc.
        cmake_layout(self)

    def build(self):
        cmake = CMake(self)
        cmake.configure()  # runs cmake with toolchain from CMakeToolchain
        cmake.build()      # compiles the app executable
