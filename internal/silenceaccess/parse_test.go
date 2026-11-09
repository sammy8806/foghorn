package silenceaccess

import "testing"

func TestParseRules(t *testing.T) {
	got, err := ParseRules([]byte(`{"data":{"groups":[{"folderUid":"g1","rules":[
		{"uid":"own","folderUid":"f1"},{"uid":"inherit"},{"name":"no uid"}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["own"] != "f1" || got["inherit"] != "g1" {
		t.Fatalf("got %v", got)
	}
}

func TestParseFolders(t *testing.T) {
	parents, titles, err := ParseFolders([]byte(`[{"uid":"top","title":"Top"},{"uid":"sub","folderUid":"top","title":"Sub"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := parents["top"]; !ok || p != "" || parents["sub"] != "top" || titles["sub"] != "Sub" {
		t.Fatalf("parents %v titles %v", parents, titles)
	}
}
