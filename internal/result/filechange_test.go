package result_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

func TestFileChangeCopyContract(t *testing.T) {
	valid := `{"packet_id":"p","status":"done","summary":"copied","hard_stops":[],"files_changed":[{"change":"copied","source_path":"old.txt","path":"new.txt"}]}`
	envelope, err := result.Read(fstest.MapFS{"result.json": {Data: []byte(valid)}}, "result.json")
	if err != nil {
		t.Fatal(err)
	}
	want := result.FileChange{Change: "copied", SourcePath: "old.txt", Path: "new.txt"}
	if got := envelope.FilesChanged[0]; got != want {
		t.Fatalf("FileChange = %+v, want %+v", got, want)
	}

	for _, invalid := range []string{
		`{"packet_id":"p","status":"done","summary":"bad","hard_stops":[],"files_changed":[{"change":"copied","path":"new.txt"}]}`,
		`{"packet_id":"p","status":"done","summary":"bad","hard_stops":[],"files_changed":[{"change":"modified","source_path":"old.txt","path":"new.txt"}]}`,
	} {
		if _, err := result.Read(fstest.MapFS{"result.json": {Data: []byte(invalid)}}, "result.json"); !errors.Is(err, result.ErrSchemaInvalid) {
			t.Fatalf("Read(invalid copy) error = %v", err)
		}
	}
}
