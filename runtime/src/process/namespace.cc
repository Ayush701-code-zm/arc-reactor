#include "tiny/process/namespace.h"

#if defined(__linux__)
#include <sched.h>
#include <unistd.h>
#endif

namespace tiny::process {

bool EnterPidNamespace(std::string* err) {
#if !defined(__linux__)
  if (err) *err = "pid namespace requires Linux";
  return false;
#else
  if (unshare(CLONE_NEWPID) != 0) {
    if (err) *err = "unshare(CLONE_NEWPID) failed";
    return false;
  }
  return true;
#endif
}

}  // namespace tiny::process
