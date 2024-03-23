#include "rpm.h"
#include <rpm/rpmdb.h>
#include <rpm/rpmlib.h>
#include <rpm/rpmts.h>

int get_installed_rpm_count() {
  int res = rpmReadConfigFiles(NULL, NULL);
  if (res == -1) {
    return 0;
  }

  rpmts ts = rpmtsCreate();

  rpmdbMatchIterator iter = rpmtsInitIterator(ts, RPMTAG_NAME, NULL, 0);
  if (iter == NULL) {
    rpmtsFree(ts);
    return 0;
  }

  int count = rpmdbGetIteratorCount(iter);

  rpmdbFreeIterator(iter);
  rpmtsFree(ts);

  return count;
}
