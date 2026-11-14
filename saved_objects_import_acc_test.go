package kibana_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccSavedObjectsExportImport(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	src := client.InSpace(createSpace(t, client))
	ipID := randName(t, "acc")
	createIndexPattern(t, src, ipID, ipID+"-*")

	// Export the index pattern, with the details line.
	var export bytes.Buffer
	exported, _, err := src.SavedObjects.Export(ctx, &export, &kibana.ExportSavedObjectsRequest{
		Objects: []kibana.SavedObjectRef{{Type: "index-pattern", ID: ipID}},
	}, nil)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if exported == nil || exported.ExportedCount != 1 || exported.MissingRefCount != 0 {
		t.Errorf("Export() details = %+v, want 1 exported object", exported)
	}
	var lines [][]byte
	sc := bufio.NewScanner(bytes.NewReader(export.Bytes()))
	for sc.Scan() {
		lines = append(lines, bytes.Clone(sc.Bytes()))
	}
	if len(lines) != 2 {
		t.Fatalf("Export() wrote %d lines, want the object and the details:\n%s", len(lines), export.String())
	}
	var details kibana.ExportDetails
	if err := json.Unmarshal(lines[1], &details); err != nil || details.ExportedCount != 1 {
		t.Errorf("export details = %+v, %v; want 1 exported object", details, err)
	}

	_, _, err = src.SavedObjects.Export(ctx, &bytes.Buffer{}, &kibana.ExportSavedObjectsRequest{
		Objects: []kibana.SavedObjectRef{{Type: "index-pattern", ID: randName(t, "missing")}},
	}, nil)
	if !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("Export(missing) error = %v, want ErrBadRequest", err)
	}

	// Import into another space; index patterns get a new ID there.
	dst := client.InSpace(createSpace(t, client))
	result, _, err := dst.SavedObjects.Import(ctx, bytes.NewReader(export.Bytes()), nil)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !result.Success || len(result.SuccessResults) != 1 || result.SuccessResults[0].ID != ipID {
		t.Fatalf("Import() = %+v, want the index pattern", result)
	}
	destID := result.SuccessResults[0].DestinationID

	// Importing again conflicts.
	result, _, err = dst.SavedObjects.Import(ctx, bytes.NewReader(export.Bytes()), nil)
	if err != nil {
		t.Fatalf("Import(again) error = %v", err)
	}
	if result.Success || len(result.Errors) != 1 || result.Errors[0].Detail == nil || result.Errors[0].Detail.Type != kibana.ImportErrorTypeConflict {
		t.Fatalf("Import(again) = %+v, want a conflict", result)
	}

	// Resolve the conflict by overwriting.
	result, _, err = dst.SavedObjects.ResolveImportErrors(ctx, bytes.NewReader(export.Bytes()), []kibana.ImportRetry{
		{Type: "index-pattern", ID: ipID, Overwrite: true, DestinationID: result.Errors[0].Detail.DestinationID},
	}, nil)
	if err != nil {
		t.Fatalf("ResolveImportErrors() error = %v", err)
	}
	if !result.Success || len(result.SuccessResults) != 1 || !result.SuccessResults[0].Overwrite || result.SuccessResults[0].DestinationID != destID {
		t.Errorf("ResolveImportErrors() = %+v, want %s overwritten", result, destID)
	}

	// New copies never conflict.
	result, _, err = dst.SavedObjects.Import(ctx, bytes.NewReader(export.Bytes()), &kibana.ImportSavedObjectsOptions{CreateNewCopies: true})
	if err != nil {
		t.Fatalf("Import(new copies) error = %v", err)
	}
	if !result.Success || result.SuccessResults[0].DestinationID == destID {
		t.Errorf("Import(new copies) = %+v, want a new copy", result)
	}
}

func TestAccEncryptedSavedObjectsRotateKey(t *testing.T) {
	client := accClient(t)
	// The compose stack configures a decryption-only key; other stacks may
	// not.
	if !localStack() {
		t.Skip("needs xpack.encryptedSavedObjects.keyRotation.decryptionOnlyKeys")
	}
	got, _, err := client.EncryptedSavedObjects.RotateKey(t.Context(), &kibana.RotateKeyOptions{BatchSize: 1000})
	if errors.Is(err, kibana.ErrTooManyRequests) {
		t.Skip("another rotation is running")
	}
	if err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}
	// The rotated objects are Kibana's own, and Kibana occasionally fails
	// to rotate one of them, so failures are not the client's.
	if got.Total == 0 || got.Successful+got.Failed > got.Total {
		t.Errorf("RotateKey() = %+v, want consistent counts", got)
	}
	t.Logf("RotateKey() = %+v", got)
}
