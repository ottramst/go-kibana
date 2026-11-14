package kibana

import (
	"context"
	"net/http"
)

// EncryptedSavedObjectsService handles communication with the encrypted
// saved object methods of the Kibana API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-saved-objects
type EncryptedSavedObjectsService service

// RotateKeyOptions are the options of [EncryptedSavedObjectsService.RotateKey].
type RotateKeyOptions struct {
	_ [0]func()

	// BatchSize is the number of objects to process at a time, at most
	// 10000 (Kibana's default).
	BatchSize int `url:"batch_size,omitempty"`

	// Type limits the rotation to the objects of this type, such as
	// "action".
	Type string `url:"type,omitempty"`
}

// RotateKeyResult is the result of [EncryptedSavedObjectsService.RotateKey].
type RotateKeyResult struct {
	_ [0]func()

	// Total is the number of encrypted objects that were found. It can be
	// larger than Successful + Failed: Kibana skips the objects that are
	// already encrypted with the current key or that no key can decrypt.
	Total int `json:"total"`

	// Successful is the number of objects that were encrypted again with
	// the current key.
	Successful int `json:"successful"`

	// Failed is the number of objects that could not be encrypted again.
	Failed int `json:"failed"`
}

// RotateKey encrypts the encrypted saved objects of all spaces again with
// the current encryption key (xpack.encryptedSavedObjects.encryptionKey),
// decrypting them with the old keys listed in
// xpack.encryptedSavedObjects.keyRotation.decryptionOnlyKeys.
//
// It needs the superuser role. Kibana answers [ErrBadRequest] when no
// decryption-only keys are configured, and [ErrTooManyRequests] while
// another rotation is running. The endpoint is in technical preview.
//
// Kibana API: POST /api/encrypted_saved_objects/_rotate_key
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-rotateencryptionkey
func (s *EncryptedSavedObjectsService) RotateKey(ctx context.Context, opt *RotateKeyOptions, opts ...RequestOption) (*RotateKeyResult, *Response, error) {
	path, err := addQuery("/api/encrypted_saved_objects/_rotate_key", opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(RotateKeyResult)
	resp, err := s.client.do(ctx, http.MethodPost, path, nil, result, nil, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
