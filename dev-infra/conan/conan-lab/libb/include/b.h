#pragma once
#include <string>

// Returns a string identifying this build of libb.
// Bump the literal below (and the `version` in conanfile.py) when you
// export the "2.0" release for the requires-mode experiment.
std::string b_version();
