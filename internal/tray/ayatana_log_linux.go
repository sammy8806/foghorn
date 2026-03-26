//go:build linux && linux_tray

package tray

/*
#cgo pkg-config: glib-2.0

#include <glib.h>
#include <string.h>

static guint foghorn_ayatana_handler_id = 0;

static void foghorn_ayatana_log_handler(const gchar *log_domain, GLogLevelFlags log_level, const gchar *message, gpointer user_data) {
	if (message != NULL && strcmp(message, "libayatana-appindicator is deprecated. Please use libayatana-appindicator-glib in newly written code.") == 0) {
		return;
	}
	g_log_default_handler(log_domain, log_level, message, user_data);
}

static void foghorn_install_ayatana_warning_filter(void) {
	if (foghorn_ayatana_handler_id != 0) {
		return;
	}
	foghorn_ayatana_handler_id = g_log_set_handler(
		"libayatana-appindicator",
		G_LOG_LEVEL_WARNING,
		foghorn_ayatana_log_handler,
		NULL
	);
}

static void foghorn_remove_ayatana_warning_filter(void) {
	if (foghorn_ayatana_handler_id == 0) {
		return;
	}
	g_log_remove_handler("libayatana-appindicator", foghorn_ayatana_handler_id);
	foghorn_ayatana_handler_id = 0;
}
*/
import "C"

func installAyatanaWarningFilter() {
	C.foghorn_install_ayatana_warning_filter()
}

func removeAyatanaWarningFilter() {
	C.foghorn_remove_ayatana_warning_filter()
}
