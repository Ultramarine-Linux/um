#include "pkg.h"
#include <flatpak/flatpak.h>
#include <rpm/rpmdb.h>
#include <rpm/rpmlib.h>
#include <rpm/rpmts.h>

int get_installed_system_flatpak_count() {
  GError *err = NULL;

  FlatpakInstallation *installation =
      flatpak_installation_new_system(NULL, &err);

  if (err != NULL) {
    g_warning("unable to get system flatpak installation: %s\n", err->message);
    g_error_free(err);
    return 0;
  }

  GPtrArray *arr =
      flatpak_installation_list_installed_refs(installation, NULL, &err);

  if (err != NULL) {
    g_warning("unable to get installed system flatpak refs: %s\n",
              err->message);
    g_error_free(err);
    return 0;
  }

  int count = arr->len;

  g_ptr_array_unref(arr);
  g_clear_object(&installation);

  return count;
}

int get_installed_user_flatpak_count() {
  GError *err = NULL;

  FlatpakInstallation *installation = flatpak_installation_new_user(NULL, &err);

  if (err != NULL) {
    g_warning("unable to get user flatpak installation: %s\n", err->message);
    g_error_free(err);
    return 0;
  }

  GPtrArray *arr =
      flatpak_installation_list_installed_refs(installation, NULL, &err);

  if (err != NULL) {
    g_warning("unable to get installed user flatpak refs: %s\n", err->message);
    g_error_free(err);
    return 0;
  }

  int count = arr->len;

  g_ptr_array_unref(arr);
  g_clear_object(&installation);

  return count;
}

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
