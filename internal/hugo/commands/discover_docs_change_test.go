package commands

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestDetectDocumentChanges tests the document change detection logic.

func TestDetectDocumentChanges_NoPreviousFiles(t *testing.T) {
	prevFiles := []docs.DocFile{}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	// When there are no previous files, changed should be false (initial state)
	if changed {
		t.Error("Expected no change detection when no previous files exist")
	}
}

func TestDetectDocumentChanges_CountChanged(t *testing.T) {
	prevFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
		{Repository: "repo1", Name: "doc3", Extension: ".md"},
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if !changed {
		t.Error("Expected change when file count differs")
	}
}

func TestDetectDocumentChanges_FileAdded(t *testing.T) {
	prevFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
		{Repository: "repo1", Name: "doc3", Extension: ".md"},
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if !changed {
		t.Error("Expected change when new file is added")
	}
}

func TestDetectDocumentChanges_FileRemoved(t *testing.T) {
	prevFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
		{Repository: "repo1", Name: "doc3", Extension: ".md"},
	}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if !changed {
		t.Error("Expected change when file is removed")
	}
}

func TestDetectDocumentChanges_FileReplaced(t *testing.T) {
	prevFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc3", Extension: ".md"}, // doc2 replaced with doc3
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if !changed {
		t.Error("Expected change when file is replaced")
	}
}

func TestDetectDocumentChanges_NoChanges(t *testing.T) {
	prevFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if changed {
		t.Error("Expected no change when files are identical")
	}
}

func TestDetectDocumentChanges_DifferentOrder(t *testing.T) {
	prevFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
		{Repository: "repo1", Name: "doc2", Extension: ".md"},
	}
	newFiles := []docs.DocFile{
		{Repository: "repo1", Name: "doc2", Extension: ".md"}, // Different order
		{Repository: "repo1", Name: "doc1", Extension: ".md"},
	}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if changed {
		t.Error("Expected no change when only order differs")
	}
}

func TestDetectDocumentChanges_EmptyLists(t *testing.T) {
	prevFiles := []docs.DocFile{}
	newFiles := []docs.DocFile{}

	changed := stages.DetectDocumentChanges(prevFiles, newFiles, false)
	if changed {
		t.Error("Expected no change when both lists are empty")
	}
}
