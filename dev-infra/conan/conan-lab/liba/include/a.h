#pragma once
#include <string>

// Returns a string identifying liba's build, plus the libb version it
// was linked against (proves which libb actually got resolved).
std::string a_version();
