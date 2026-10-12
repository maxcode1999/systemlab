// libb implementation: version string must stay in sync with conanfile.py `version`
// when you export libb/2.0 for the lockfile / package_id experiments.
#include "b.h"

std::string b_version() {
    return "libb 1.0";   // change to "libb 2.0" for the version-2 export
}
