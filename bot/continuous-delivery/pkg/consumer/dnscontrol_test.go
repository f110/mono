package consumer

import (
	"testing"
)

func TestChangedFiles(t *testing.T) {
	d := `diff --git a/README.md b/README.md
index 8b30266..c7dd277 100644
--- a/README.md
+++ b/README.md
@@ -1 +1,5 @@
-# bot-staging
\ No newline at end of file
+# bot-staging
+
+# Author
+
+Fumihiro Ito`

	files, err := changedFilesFromDiff(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("Expect 1 file: %d files", len(files))
	}
	if files[0] != "/README.md" {
		t.Errorf("Expect /README.md: %s", files[0])
	}
}

func TestExtractPRNumberFromMergedMessage(t *testing.T) {
	num := extractPRNumberFromMergedMessage("Merge pull request #2 from f110/pr-test\n\nPR Test")
	if num != 2 {
		t.Errorf("Expect 2: %d", num)
	}

	num = extractPRNumberFromMergedMessage("Merge pull request #10000 from f110/pr-test\n\nPR Test")
	if num != 10000 {
		t.Errorf("Expect 10000: %d", num)
	}
}
