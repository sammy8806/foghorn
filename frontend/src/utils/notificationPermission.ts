export interface NotificationPermissionActionResult {
  status: string;
  error: string;
}

const allowedStatuses = new Set(['authorized', 'provisional', 'ephemeral']);

export function isNotificationPermissionAllowed(status: string): boolean {
  return allowedStatuses.has(status);
}

// Ask for authorization before opening Settings so macOS has registered the
// app as a notification client. If the request is denied or fails, Settings is
// still opened so the user has a path to recover.
export async function requestNotificationPermissionOrOpenSettings(
  currentStatus: string,
  requestPermission: () => Promise<string>,
  openSettings: () => Promise<void>,
): Promise<NotificationPermissionActionResult> {
  let status = currentStatus;
  const errors: string[] = [];

  if (status === 'not_determined') {
    try {
      status = await requestPermission();
      if (isNotificationPermissionAllowed(status)) {
        return { status, error: '' };
      }
    } catch (error) {
      errors.push(String(error));
    }
  }

  try {
    await openSettings();
  } catch (error) {
    errors.push(String(error));
  }

  return { status, error: errors.join('; ') };
}
