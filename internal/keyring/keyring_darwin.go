//go:build darwin && cgo

package keyring

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

static CFStringRef foghorn_cf_string(const char *value) {
	return CFStringCreateWithCString(kCFAllocatorDefault, value, kCFStringEncodingUTF8);
}

static CFMutableDictionaryRef foghorn_query(const char *service, const char *account) {
	CFStringRef serviceValue = foghorn_cf_string(service);
	CFStringRef accountValue = foghorn_cf_string(account);
	if (serviceValue == NULL || accountValue == NULL) {
		if (serviceValue != NULL) CFRelease(serviceValue);
		if (accountValue != NULL) CFRelease(accountValue);
		return NULL;
	}
	CFMutableDictionaryRef query = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	if (query != NULL) {
		CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(query, kSecAttrService, serviceValue);
		CFDictionarySetValue(query, kSecAttrAccount, accountValue);
		CFDictionarySetValue(query, kSecAttrSynchronizable, kCFBooleanFalse);
	}
	CFRelease(serviceValue);
	CFRelease(accountValue);
	return query;
}

static OSStatus foghorn_keyring_get(
	const char *service,
	const char *account,
	void **bytes,
	CFIndex *length
) {
	*bytes = NULL;
	*length = 0;
	CFMutableDictionaryRef query = foghorn_query(service, account);
	if (query == NULL) return errSecAllocate;
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);

	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	if (status != errSecSuccess) return status;
	if (result == NULL || CFGetTypeID(result) != CFDataGetTypeID()) {
		if (result != NULL) CFRelease(result);
		return errSecDecode;
	}

	CFDataRef data = (CFDataRef)result;
	CFIndex dataLength = CFDataGetLength(data);
	if (dataLength > 0) {
		void *copy = malloc((size_t)dataLength);
		if (copy == NULL) {
			CFRelease(result);
			return errSecAllocate;
		}
		memcpy(copy, CFDataGetBytePtr(data), (size_t)dataLength);
		*bytes = copy;
	}
	*length = dataLength;
	CFRelease(result);
	return errSecSuccess;
}

static OSStatus foghorn_keyring_set(
	const char *service,
	const char *account,
	const void *bytes,
	CFIndex length
) {
	CFDataRef data = CFDataCreate(kCFAllocatorDefault, bytes, length);
	if (data == NULL) return errSecAllocate;
	CFMutableDictionaryRef query = foghorn_query(service, account);
	if (query == NULL) {
		CFRelease(data);
		return errSecAllocate;
	}

	CFMutableDictionaryRef updates = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	if (updates == NULL) {
		CFRelease(query);
		CFRelease(data);
		return errSecAllocate;
	}
	CFDictionarySetValue(updates, kSecValueData, data);
	CFDictionarySetValue(updates, kSecAttrAccessible, kSecAttrAccessibleAfterFirstUnlock);

	OSStatus status = SecItemUpdate(query, updates);
	if (status == errSecItemNotFound) {
		CFDictionarySetValue(query, kSecValueData, data);
		CFDictionarySetValue(query, kSecAttrAccessible, kSecAttrAccessibleAfterFirstUnlock);
		status = SecItemAdd(query, NULL);
	}
	CFRelease(updates);
	CFRelease(query);
	CFRelease(data);
	return status;
}

static OSStatus foghorn_keyring_delete(const char *service, const char *account) {
	CFMutableDictionaryRef query = foghorn_query(service, account);
	if (query == NULL) return errSecAllocate;
	OSStatus status = SecItemDelete(query);
	CFRelease(query);
	return status;
}

static char *foghorn_keyring_error(OSStatus status) {
	CFStringRef message = SecCopyErrorMessageString(status, NULL);
	if (message == NULL) return NULL;
	CFIndex length = CFStringGetMaximumSizeForEncoding(
		CFStringGetLength(message), kCFStringEncodingUTF8
	) + 1;
	char *buffer = malloc((size_t)length);
	if (buffer == NULL) {
		CFRelease(message);
		return NULL;
	}
	if (!CFStringGetCString(message, buffer, length, kCFStringEncodingUTF8)) {
		free(buffer);
		buffer = NULL;
	}
	CFRelease(message);
	return buffer;
}

static void foghorn_clear_and_free(void *bytes, size_t length) {
	if (bytes == NULL) return;
	volatile unsigned char *cursor = (volatile unsigned char *)bytes;
	while (length-- > 0) *cursor++ = 0;
	free(bytes);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type macOSStore struct {
	service string
}

func newStore(service string) Store {
	return &macOSStore{service: service}
}

func supported() bool { return true }

func (s *macOSStore) Get(account string) ([]byte, error) {
	serviceValue := C.CString(s.service)
	accountValue := C.CString(account)
	defer C.free(unsafe.Pointer(serviceValue))
	defer C.free(unsafe.Pointer(accountValue))

	var bytes unsafe.Pointer
	var length C.CFIndex
	status := C.foghorn_keyring_get(serviceValue, accountValue, &bytes, &length)
	if status != C.errSecSuccess {
		return nil, keyringError("read", status)
	}
	if length == 0 {
		return nil, ErrNotFound
	}
	if bytes != nil {
		defer C.foghorn_clear_and_free(bytes, C.size_t(length))
	}
	return C.GoBytes(bytes, C.int(length)), nil
}

func (s *macOSStore) Set(account string, secret []byte) error {
	serviceValue := C.CString(s.service)
	accountValue := C.CString(account)
	secretValue := C.CBytes(secret)
	defer C.free(unsafe.Pointer(serviceValue))
	defer C.free(unsafe.Pointer(accountValue))
	defer C.foghorn_clear_and_free(secretValue, C.size_t(len(secret)))

	status := C.foghorn_keyring_set(
		serviceValue,
		accountValue,
		secretValue,
		C.CFIndex(len(secret)),
	)
	if status != C.errSecSuccess {
		return keyringError("save", status)
	}
	return nil
}

func (s *macOSStore) Delete(account string) error {
	serviceValue := C.CString(s.service)
	accountValue := C.CString(account)
	defer C.free(unsafe.Pointer(serviceValue))
	defer C.free(unsafe.Pointer(accountValue))

	status := C.foghorn_keyring_delete(serviceValue, accountValue)
	if status == C.errSecItemNotFound {
		return nil
	}
	if status != C.errSecSuccess {
		return keyringError("delete", status)
	}
	return nil
}

func keyringError(operation string, status C.OSStatus) error {
	if status == C.errSecItemNotFound {
		return ErrNotFound
	}
	messageValue := C.foghorn_keyring_error(status)
	if messageValue == nil {
		return fmt.Errorf("keyring %s failed (OSStatus %d)", operation, int32(status))
	}
	defer C.free(unsafe.Pointer(messageValue))
	message := C.GoString(messageValue)
	if message == "" {
		return fmt.Errorf("keyring %s failed (OSStatus %d)", operation, int32(status))
	}
	return fmt.Errorf("keyring %s failed (OSStatus %d): %s", operation, int32(status), message)
}
