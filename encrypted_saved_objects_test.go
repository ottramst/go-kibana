package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestEncryptedSavedObjectsService_RotateKey(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/encrypted_saved_objects/_rotate_key", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testQuery(t, r, "batch_size=100&type=action")
		writeJSON(t, w, http.StatusOK, fixture(t, "encryptedsavedobjects/rotate.json"))
	})
	got, _, err := client.EncryptedSavedObjects.RotateKey(context.Background(), &kibana.RotateKeyOptions{BatchSize: 100, Type: "action"})
	if err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}
	if want := (kibana.RotateKeyResult{Total: 75, Successful: 74}); !reflect.DeepEqual(*got, want) {
		t.Errorf("RotateKey() = %+v, want %+v", *got, want)
	}
}

func TestEncryptedSavedObjectsService_RotateKey_errors(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		want    error
	}{
		{"encryptedsavedobjects/rotate_in_progress.json", http.StatusTooManyRequests, kibana.ErrTooManyRequests},
		{"encryptedsavedobjects/rotate_bigbatch.json", http.StatusBadRequest, kibana.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /s/x/api/encrypted_saved_objects/_rotate_key", func(w http.ResponseWriter, r *http.Request) {
				testQuery(t, r, "")
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			got, _, err := client.InSpace("x").EncryptedSavedObjects.RotateKey(context.Background(), nil)
			if !errors.Is(err, tt.want) || got != nil {
				t.Errorf("RotateKey() = %v, %v; want nil, %v", got, err, tt.want)
			}
		})
	}
}
