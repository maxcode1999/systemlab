// Lab executable: links liba (which links libb). Prints version strings and
// the glibc version at runtime — used in Phase 5 when copying binaries across VMs.
#include <gnu/libc-version.h>
#include <iostream>

#include "a.h"

int main() {
    std::cout << "app running\n";
    std::cout << "  " << a_version() << "\n";
    std::cout << "  glibc version at runtime: " << gnu_get_libc_version() << "\n";
    return 0;
}
