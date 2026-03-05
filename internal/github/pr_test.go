package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- parseRemoteURL tests ----

func TestParseRemoteURL_HTTPS(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "plain HTTPS",
			remote:    "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "HTTPS with .git suffix",
			remote:    "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "trailing whitespace",
			remote:    "https://github.com/JosephLeKH/coderev.git\n",
			wantOwner: "JosephLeKH",
			wantRepo:  "coderev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRemoteURL(tt.remote)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestParseRemoteURL_SSH(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "SSH without .git",
			remote:    "git@github.com:owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "SSH with .git suffix",
			remote:    "git@github.com:JosephLeKH/coderev.git",
			wantOwner: "JosephLeKH",
			wantRepo:  "coderev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRemoteURL(tt.remote)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestParseRemoteURL_NonGitHub(t *testing.T) {
	nonGitHub := []string{
		"https://gitlab.com/owner/repo.git",
		"git@bitbucket.org:owner/repo.git",
		"https://example.com/repo",
		"not-a-url",
	}
	for _, remote := range nonGitHub {
		t.Run(remote, func(t *testing.T) {
			_, _, err := parseRemoteURL(remote)
			assert.Error(t, err)
		})
	}
}

// ---- buildPositionMap tests ----

// diffContent simulates what FileChunk.Content looks like for a small file edit.
// The content comes AFTER the "diff --git" line, so it starts with the index/---/+++ headers.
const singleHunkDiff = `index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main

+import "fmt"

 func main() {
-	println("hello")
+	fmt.Println("hello")
 }`

func TestBuildPositionMap_SingleHunk(t *testing.T) {
	// Diff:
	//   @@ -1,5 +1,6 @@        ← not counted
	//    package main           ← position 1, new line 1
	//                           ← position 2, new line 2
	//   +import "fmt"          ← position 3, new line 3   ← added
	//                           ← position 4, new line 4
	//    func main() {         ← position 5, new line 5
	//   -	println("hello")  ← position 6,  (removed, no new line)
	//   +	fmt.Println("hello")← position 7, new line 6   ← added
	//    }                     ← position 8, new line 7

	posMap := buildPositionMap(singleHunkDiff)

	// "import fmt" is the first added line → new file line 3
	assert.Equal(t, 3, posMap[3], "import line should be at position 3")

	// "fmt.Println" replaces "println" → new file line 6
	assert.Equal(t, 7, posMap[6], "fmt.Println line should be at position 7")

	// A context line (package main) should NOT be in the posMap (only + lines are mapped)
	_, exists := posMap[1]
	assert.False(t, exists, "context lines should not appear in posMap")
}

const multiHunkDiff = `index abc1234..def5678 100644
--- a/util.go
+++ b/util.go
@@ -1,3 +1,4 @@
 package util

+// Added comment
 func Foo() {}
@@ -10,3 +11,4 @@
 func Bar() {}

+// Another comment
 func Baz() {}
`

func TestBuildPositionMap_MultiHunk(t *testing.T) {
	// First hunk (@@ -1,3 +1,4 @@):
	//   position 1: " package util"      (new line 1)
	//   position 2: " "                  (new line 2)
	//   position 3: "+// Added comment"  (new line 3) ← added
	//   position 4: " func Foo() {}"     (new line 4)
	//
	// Second hunk (@@ -10,3 +11,4 @@):
	//   position 5: " func Bar() {}"     (new line 11)
	//   position 6: " "                  (new line 12)
	//   position 7: "+// Another comment"(new line 13) ← added
	//   position 8: " func Baz() {}"     (new line 14)

	posMap := buildPositionMap(multiHunkDiff)

	assert.Equal(t, 3, posMap[3], "first added line should be at position 3")
	assert.Equal(t, 7, posMap[13], "second added line should be at position 7")
}

func TestBuildPositionMap_EmptyContent(t *testing.T) {
	posMap := buildPositionMap("")
	assert.Empty(t, posMap)
}

func TestBuildPositionMap_NoAddedLines(t *testing.T) {
	// Diff with only removed lines — no + lines, posMap should be empty.
	onlyRemovals := `index abc..def 100644
--- a/x.go
+++ b/x.go
@@ -1,3 +1,2 @@
 package x
-// remove me
 func X() {}
`
	posMap := buildPositionMap(onlyRemovals)
	// Only context lines and removed lines — no added lines to map.
	assert.Empty(t, posMap)
}
