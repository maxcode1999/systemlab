// Lab A1 benchmark: 4K vs MAP_HUGETLB vs THP (MADV_HUGEPAGE).
// Linux x86_64; uses rdtscp for phase timing when available.

#define _GNU_SOURCE
#include <cerrno>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <chrono>
#include <string>

#include <sys/mman.h>
#include <unistd.h>

#ifndef MAP_HUGETLB
#define MAP_HUGETLB 0x40000
#endif
#ifndef MADV_HUGEPAGE
#define MADV_HUGEPAGE 14
#endif

#if defined(__x86_64__) || defined(__i386__)
#include <x86intrin.h>
#define HAVE_RDTSCP 1
#endif

static uint64_t read_tsc() {
#ifdef HAVE_RDTSCP
  unsigned int aux = 0;
  _mm_mfence();
  return __rdtscp(&aux);
#else
  return static_cast<uint64_t>(
      std::chrono::steady_clock::now().time_since_epoch().count());
#endif
}

enum class Mode { k4k, kHugetlb, kThp };

static Mode parse_mode(const char* s) {
  if (std::strcmp(s, "4k") == 0) return Mode::k4k;
  if (std::strcmp(s, "hugetlb") == 0) return Mode::kHugetlb;
  if (std::strcmp(s, "thp") == 0) return Mode::kThp;
  std::fprintf(stderr, "unknown mode: %s (use 4k | hugetlb | thp)\n", s);
  std::exit(1);
}

static const char* mode_str(Mode m) {
  switch (m) {
    case Mode::k4k: return "4k";
    case Mode::kHugetlb: return "hugetlb";
    case Mode::kThp: return "thp";
  }
  return "?";
}

static void* map_region(Mode mode, size_t size) {
  int flags = MAP_PRIVATE | MAP_ANONYMOUS;
  if (mode == Mode::kHugetlb) flags |= MAP_HUGETLB;

  void* p = mmap(nullptr, size, PROT_READ | PROT_WRITE, flags, -1, 0);
  if (p == MAP_FAILED) {
    std::perror("mmap");
    std::exit(1);
  }

  if (mode == Mode::kThp) {
    if (madvise(p, size, MADV_HUGEPAGE) != 0) {
      std::perror("madvise(MADV_HUGEPAGE)");
      munmap(p, size);
      std::exit(1);
    }
  }
  return p;
}

static void touch_sequential(volatile char* p, size_t size, size_t stride) {
  for (size_t off = 0; off < size; off += stride) p[off] = static_cast<char>(off);
}

static uint64_t random_pass(volatile char* p, size_t size, uint64_t iterations,
                            size_t stride) {
  const size_t slots = size / stride;
  if (slots == 0) return 0;

  uint64_t state = 0x9e3779b97f4a7c15ULL;
  auto next = [&state]() -> uint64_t {
    state ^= state >> 12;
    state ^= state << 25;
    state ^= state >> 27;
    return state * 0x2545F4914F6CDD1DULL;
  };

  volatile uint64_t sink = 0;
  const uint64_t t0 = read_tsc();
  for (uint64_t i = 0; i < iterations; ++i) {
    const size_t idx = static_cast<size_t>(next() % slots);
    sink += static_cast<unsigned char>(p[idx * stride]);
  }
  const uint64_t t1 = read_tsc();
  if (sink == 0xdeadbeefULL) {
    std::fprintf(stderr, "%llu\n", static_cast<unsigned long long>(sink));
  }
  return t1 - t0;
}

static void print_timing(const char* mode, const char* phase, size_t size,
                         uint64_t cycles, uint64_t extra = 0) {
  std::printf("mode=%s phase=%s size=%zu rdtscp_cycles=%llu",
              mode, phase, size,
              static_cast<unsigned long long>(cycles));
  if (extra) std::printf(" iterations=%llu", static_cast<unsigned long long>(extra));
  std::printf("\n");
  std::fflush(stdout);
}

static void usage(const char* argv0) {
  std::fprintf(stderr,
               "Usage: %s --mode 4k|hugetlb|thp --size BYTES "
               "[--touch-only | --random-pass | --all]\n"
               "       [--iterations N]  (default 50000000 for random)\n"
               "       [--stride B]      (touch/random step; default 4096)\n",
               argv0);
}

int main(int argc, char** argv) {
  Mode mode = Mode::k4k;
  size_t size = 0;
  size_t stride = 4096;
  uint64_t iterations = 50'000'000;
  bool do_touch = false;
  bool do_random = false;

  for (int i = 1; i < argc; ++i) {
    if (std::strcmp(argv[i], "--mode") == 0 && i + 1 < argc) {
      mode = parse_mode(argv[++i]);
    } else if (std::strcmp(argv[i], "--size") == 0 && i + 1 < argc) {
      size = std::strtoull(argv[++i], nullptr, 10);
    } else if (std::strcmp(argv[i], "--stride") == 0 && i + 1 < argc) {
      stride = static_cast<size_t>(std::strtoull(argv[++i], nullptr, 10));
    } else if (std::strcmp(argv[i], "--iterations") == 0 && i + 1 < argc) {
      iterations = std::strtoull(argv[++i], nullptr, 10);
    } else if (std::strcmp(argv[i], "--touch-only") == 0) {
      do_touch = true;
    } else if (std::strcmp(argv[i], "--random-pass") == 0) {
      do_random = true;
    } else if (std::strcmp(argv[i], "--all") == 0) {
      do_touch = true;
      do_random = true;
    } else {
      usage(argv[0]);
      return 1;
    }
  }

  if (size == 0) {
    usage(argv[0]);
    return 1;
  }
  if (!do_touch && !do_random) {
    do_touch = true;
    do_random = true;
  }
  if (stride == 0 || size < stride) {
    std::fprintf(stderr, "invalid size/stride\n");
    return 1;
  }

  void* raw = map_region(mode, size);
  volatile char* p = static_cast<volatile char*>(raw);
  const char* m = mode_str(mode);

  if (do_touch) {
    const uint64_t t0 = read_tsc();
    touch_sequential(p, size, stride);
    const uint64_t t1 = read_tsc();
    print_timing(m, "touch", size, t1 - t0);
  }

  if (do_random) {
    if (!do_touch) {
      touch_sequential(p, size, stride);
    }
    const uint64_t cyc = random_pass(p, size, iterations, stride);
    print_timing(m, "random", size, cyc, iterations);
  }

  if (munmap(raw, size) != 0) std::perror("munmap");
  return 0;
}
