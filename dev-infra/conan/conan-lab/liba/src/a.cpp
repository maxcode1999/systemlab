// liba implementation: forwards version info and exposes which libb was linked.
#include "a.h"
#include "b.h"

std::string a_version() {
    return "liba 1.0 (linked against " + b_version() + ")";
}
