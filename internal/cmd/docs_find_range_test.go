package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/api/docs/v1"
)

func TestFindDocsTextRangesAllOccurrenceCaseAndUTF16(t *testing.T) {
	t.Parallel()

	doc := docsFindRangeDoc(
		docsFindRangeParagraph(1,
			"Hi ",
			"😀 needle ",
			"NEEDLE",
			"\n",
		),
	)

	matches := findDocsTextRanges(doc, "needle", docsTextRangeOptions{})
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2: %#v", len(matches), matches)
	}
	if matches[0].StartIndex != 7 || matches[0].EndIndex != 13 {
		t.Fatalf("first range = %d..%d, want 7..13", matches[0].StartIndex, matches[0].EndIndex)
	}
	if matches[1].StartIndex != 14 || matches[1].EndIndex != 20 {
		t.Fatalf("second range = %d..%d, want 14..20", matches[1].StartIndex, matches[1].EndIndex)
	}

	caseMatches := findDocsTextRanges(doc, "needle", docsTextRangeOptions{MatchCase: true})
	if len(caseMatches) != 1 || caseMatches[0].StartIndex != 7 {
		t.Fatalf("case-sensitive matches = %#v, want first match only", caseMatches)
	}
}

func TestFindDocsTextRangesNormalizesWhitespaceAndEntities(t *testing.T) {
	t.Parallel()

	doc := docsFindRangeDoc(
		docsFindRangeParagraph(1,
			"Tom",
			"\t  & Jerry\n",
			"next",
		),
	)

	matches := findDocsTextRanges(doc, "Tom &amp; Jerry next", docsTextRangeOptions{
		NormalizeWhitespace: true,
	})
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1: %#v", len(matches), matches)
	}
	if got := matches[0]; got.StartIndex != 1 || got.EndIndex != 19 {
		t.Fatalf("range = %d..%d, want 1..19", got.StartIndex, got.EndIndex)
	}

	noNormalize := findDocsTextRanges(doc, "Tom & Jerry next", docsTextRangeOptions{NormalizeWhitespace: false})
	if len(noNormalize) != 0 {
		t.Fatalf("no-normalize matches = %#v, want none", noNormalize)
	}
}

func TestFindDocsTextRangesTablesAndParagraphIndex(t *testing.T) {
	t.Parallel()

	doc := &docs.Document{Body: &docs.Body{Content: []*docs.StructuralElement{
		docsFindRangeParagraph(1, "before\n"),
		{
			StartIndex: 8,
			EndIndex:   40,
			Table: &docs.Table{TableRows: []*docs.TableRow{
				{TableCells: []*docs.TableCell{
					{Content: []*docs.StructuralElement{
						docsFindRangeParagraph(10, "cell target\n"),
					}},
				}},
			}},
		},
		docsFindRangeParagraph(40, "after target\n"),
	}}}

	matches := findDocsTextRanges(doc, "target", docsTextRangeOptions{})
	if len(matches) != 2 {
		t.Fatalf("matches = %#v, want 2", matches)
	}
	if matches[0].ParagraphIndex != 1 || matches[0].StartIndex != 15 {
		t.Fatalf("table match = %#v, want paragraphIndex=1 start=15", matches[0])
	}
	if matches[1].ParagraphIndex != 2 || matches[1].StartIndex != 46 {
		t.Fatalf("body match = %#v, want paragraphIndex=2 start=46", matches[1])
	}
}

func TestFindDocsTextRangesAcrossParagraphs(t *testing.T) {
	t.Parallel()

	doc := docsFindRangeDoc(
		docsFindRangeParagraph(1, "First paragraph\n"),
		docsFindRangeParagraph(17, "Second paragraph\n"),
	)

	matches := findDocsTextRanges(doc, "paragraph Second paragraph", docsTextRangeOptions{NormalizeWhitespace: true})
	if len(matches) != 1 {
		t.Fatalf("matches = %#v, want 1", matches)
	}
	if got := matches[0]; got.StartIndex != 7 || got.EndIndex != 33 || got.ParagraphIndex != 0 {
		t.Fatalf("match = %#v, want start=7 end=33 paragraphIndex=0", got)
	}
}

func TestDocsFindRangeCmdJSONAllAndTab(t *testing.T) {
	t.Parallel()

	var includeTabs string
	docSvc, cleanup := newDocsServiceForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/v1/documents/") {
			http.NotFound(w, r)
			return
		}
		includeTabs = r.URL.Query().Get("includeTabsContent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&docs.Document{
			DocumentId: "doc1",
			Tabs: []*docs.Tab{
				{
					TabProperties: &docs.TabProperties{TabId: "t.first", Title: "First"},
					DocumentTab:   &docs.DocumentTab{Body: docsFindRangeDoc(docsFindRangeParagraph(1, "nope\n")).Body},
				},
				{
					TabProperties: &docs.TabProperties{TabId: "t.second", Title: "Second"},
					DocumentTab: &docs.DocumentTab{Body: docsFindRangeDoc(
						docsFindRangeParagraph(1, "Alpha Beta Alpha\n"),
					).Body},
				},
			},
		})
	}))
	defer cleanup()

	var output bytes.Buffer
	ctx := withDocsTestService(newCmdRuntimeJSONOutputContext(t, &output, io.Discard), docSvc)
	if err := runKong(t, &DocsFindRangeCmd{}, []string{"doc1", "Alpha", "--all", "--tab", "Second"}, ctx, &RootFlags{Account: "a@b.com"}); err != nil {
		t.Fatalf("find-range: %v", err)
	}
	out := output.String()
	if includeTabs != "true" {
		t.Fatalf("includeTabsContent = %q, want true", includeTabs)
	}

	var result docsFindRangeResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(result.Matches) != 2 {
		t.Fatalf("matches = %#v, want 2", result.Matches)
	}
	if got := result.Matches[0]; got.StartIndex != 1 || got.EndIndex != 6 || got.ParagraphIndex != 0 || got.TabID != "t.second" {
		t.Fatalf("first match = %#v", got)
	}
	if got := result.Matches[1]; got.StartIndex != 12 || got.EndIndex != 17 || got.TabID != "t.second" {
		t.Fatalf("second match = %#v", got)
	}
}

func TestDocsFindRangeCmdPlainOccurrence(t *testing.T) {
	t.Parallel()

	docSvc, cleanup := newDocsServiceForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/v1/documents/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(docsFindRangeDoc(docsFindRangeParagraph(1, "Alpha Beta Alpha\n")))
	}))
	defer cleanup()

	var out bytes.Buffer
	ctx := withDocsTestService(newCmdRuntimeOutputContext(t, &out, io.Discard), docSvc)
	if err := runKong(t, &DocsFindRangeCmd{}, []string{"doc1", "Alpha", "--occurrence", "2"}, ctx, &RootFlags{Account: "a@b.com"}); err != nil {
		t.Fatalf("find-range: %v", err)
	}
	if got, want := out.String(), "12\t17\t0\t\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestDocsFindRangeCmdEmptyAndFailEmpty(t *testing.T) {
	t.Parallel()

	docSvc, cleanup := newDocsServiceForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/v1/documents/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(docsFindRangeDoc(docsFindRangeParagraph(1, "Alpha\n")))
	}))
	defer cleanup()

	var output bytes.Buffer
	ctx := withDocsTestService(newCmdRuntimeJSONOutputContext(t, &output, io.Discard), docSvc)
	if err := runKong(t, &DocsFindRangeCmd{}, []string{"doc1", "missing"}, ctx, &RootFlags{Account: "a@b.com"}); err != nil {
		t.Fatalf("find-range empty: %v", err)
	}
	assertEmptyFindRangeJSON(t, output.String())

	output.Reset()
	runErr := runKong(t, &DocsFindRangeCmd{}, []string{"doc1", "missing", "--fail-empty"}, ctx, &RootFlags{Account: "a@b.com"})
	var exitErr *ExitError
	if !errors.As(runErr, &exitErr) || exitErr.Code != emptyResultsExitCode {
		t.Fatalf("fail-empty err = %#v, want exit 3", runErr)
	}
	assertEmptyFindRangeJSON(t, output.String())
}

func assertEmptyFindRangeJSON(t *testing.T, raw string) {
	t.Helper()
	var result docsFindRangeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if len(result.Matches) != 0 {
		t.Fatalf("matches = %#v, want empty", result.Matches)
	}
}

func docsFindRangeDoc(elements ...*docs.StructuralElement) *docs.Document {
	return &docs.Document{
		DocumentId: "doc1",
		Body:       &docs.Body{Content: elements},
	}
}

func docsFindRangeParagraph(start int64, parts ...string) *docs.StructuralElement {
	el := &docs.StructuralElement{
		StartIndex: start,
		Paragraph:  &docs.Paragraph{},
	}
	index := start
	for _, part := range parts {
		end := index + utf16Len(part)
		el.Paragraph.Elements = append(el.Paragraph.Elements, &docs.ParagraphElement{
			StartIndex: index,
			EndIndex:   end,
			TextRun:    &docs.TextRun{Content: part},
		})
		index = end
	}
	el.EndIndex = index
	return el
}
