import { describe, it, expect } from 'vitest';
import { safeExternalURL, isSafeExternalURL } from './url';

describe('safeExternalURL', () => {
  it('accepts http and https URLs', () => {
    expect(safeExternalURL('https://example.com/graph?a=b')).toBe('https://example.com/graph?a=b');
    expect(safeExternalURL('http://localhost:9093/#/alerts')).toBe('http://localhost:9093/#/alerts');
  });

  it('accepts uppercase schemes and normalises them', () => {
    expect(safeExternalURL('HTTPS://example.com/page')).toBe('https://example.com/page');
  });

  it('rejects javascript: URLs in any casing or spacing', () => {
    expect(safeExternalURL('javascript:alert(1)')).toBeNull();
    expect(safeExternalURL('JavaScript:alert(1)')).toBeNull();
    expect(safeExternalURL('  javascript:alert(1)')).toBeNull();
    expect(safeExternalURL('java\tscript:alert(1)')).toBeNull();
  });

  it('rejects other local-reach schemes', () => {
    expect(safeExternalURL('file:///etc/passwd')).toBeNull();
    expect(safeExternalURL('data:text/html,<script>alert(1)</script>')).toBeNull();
    expect(safeExternalURL('smb://attacker.example/share')).toBeNull();
    expect(safeExternalURL('ms-msdt:/id')).toBeNull();
  });

  it('rejects relative and empty values', () => {
    expect(safeExternalURL('/relative')).toBeNull();
    expect(safeExternalURL('')).toBeNull();
    expect(safeExternalURL('   ')).toBeNull();
    expect(safeExternalURL(null)).toBeNull();
    expect(safeExternalURL(undefined)).toBeNull();
  });

  it('exposes a boolean helper', () => {
    expect(isSafeExternalURL('https://example.com')).toBe(true);
    expect(isSafeExternalURL('javascript:alert(1)')).toBe(false);
  });
});
