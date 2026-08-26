import { describe, expect, it, vi } from 'vitest';
import {
  isNotificationPermissionAllowed,
  requestNotificationPermissionOrOpenSettings,
} from './notificationPermission';

describe('requestNotificationPermissionOrOpenSettings', () => {
  it('requests undetermined permission and stops after authorization', async () => {
    const requestPermission = vi.fn().mockResolvedValue('authorized');
    const openSettings = vi.fn().mockResolvedValue(undefined);

    const result = await requestNotificationPermissionOrOpenSettings(
      'not_determined',
      requestPermission,
      openSettings,
    );

    expect(result).toEqual({ status: 'authorized', error: '' });
    expect(requestPermission).toHaveBeenCalledOnce();
    expect(openSettings).not.toHaveBeenCalled();
  });

  it('opens Settings after permission is denied', async () => {
    const requestPermission = vi.fn().mockResolvedValue('denied');
    const openSettings = vi.fn().mockResolvedValue(undefined);

    const result = await requestNotificationPermissionOrOpenSettings(
      'not_determined',
      requestPermission,
      openSettings,
    );

    expect(result).toEqual({ status: 'denied', error: '' });
    expect(openSettings).toHaveBeenCalledOnce();
  });

  it('opens Settings directly for an existing denial', async () => {
    const requestPermission = vi.fn().mockResolvedValue('authorized');
    const openSettings = vi.fn().mockResolvedValue(undefined);

    const result = await requestNotificationPermissionOrOpenSettings(
      'denied',
      requestPermission,
      openSettings,
    );

    expect(result).toEqual({ status: 'denied', error: '' });
    expect(requestPermission).not.toHaveBeenCalled();
    expect(openSettings).toHaveBeenCalledOnce();
  });

  it('falls back to Settings and reports a request error', async () => {
    const requestPermission = vi.fn().mockRejectedValue(new Error('request failed'));
    const openSettings = vi.fn().mockResolvedValue(undefined);

    const result = await requestNotificationPermissionOrOpenSettings(
      'not_determined',
      requestPermission,
      openSettings,
    );

    expect(result.status).toBe('not_determined');
    expect(result.error).toContain('request failed');
    expect(openSettings).toHaveBeenCalledOnce();
  });

  it('reports a Settings launch error', async () => {
    const result = await requestNotificationPermissionOrOpenSettings(
      'denied',
      vi.fn(),
      vi.fn().mockRejectedValue(new Error('open failed')),
    );

    expect(result.error).toContain('open failed');
  });
});

describe('isNotificationPermissionAllowed', () => {
  it('accepts every status that can deliver notifications', () => {
    expect(isNotificationPermissionAllowed('authorized')).toBe(true);
    expect(isNotificationPermissionAllowed('provisional')).toBe(true);
    expect(isNotificationPermissionAllowed('ephemeral')).toBe(true);
    expect(isNotificationPermissionAllowed('denied')).toBe(false);
  });
});
