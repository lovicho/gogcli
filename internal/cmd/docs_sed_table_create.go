package cmd

import (
	"context"
	"fmt"

	"google.golang.org/api/docs/v1"

	"github.com/steipete/gogcli/internal/docssed"
	"github.com/steipete/gogcli/internal/ui"
)

type tableCreateSpec struct {
	rows   int
	cols   int
	header bool       // pin first row as header
	cells  [][]string // optional cell content for pipe-table syntax
}

func parseTableFromPipes(s string) *tableCreateSpec {
	return tableCreateSpecFromParsed(docssed.ParsePipeTable(s))
}

func parseTableCreate(s string) *tableCreateSpec {
	return tableCreateSpecFromParsed(docssed.ParseTableCreate(s))
}

func tableCreateSpecFromParsed(spec *docssed.TableCreateSpec) *tableCreateSpec {
	if spec == nil {
		return nil
	}
	return &tableCreateSpec{
		rows:   spec.Rows,
		cols:   spec.Columns,
		header: spec.Header,
		cells:  spec.Cells,
	}
}

// runTableCreate handles creating a table at the location of matched text
// fillTableCells populates a newly-created table with cell content from spec.cells.
// nearIndex is the approximate document index where the table was inserted.
func (c *DocsSedCmd) fillTableCells(ctx context.Context, docsSvc *docs.Service, id string, nearIndex int64, spec *tableCreateSpec) error {
	var doc *docs.Document
	err := retryOnQuota(ctx, func() error {
		var e error
		doc, e = docsSvc.Documents.Get(id).Context(ctx).Do()
		return e
	})
	if err != nil {
		return fmt.Errorf("re-fetch document after table create: %w", err)
	}

	tables := collectAllTables(doc)
	var targetTable *docs.Table
	for _, t := range tables {
		if len(t.TableRows) > 0 && len(t.TableRows[0].TableCells) > 0 {
			firstCell := t.TableRows[0].TableCells[0]
			if len(firstCell.Content) > 0 {
				cellStart := firstCell.Content[0].StartIndex
				if cellStart >= nearIndex && cellStart <= nearIndex+10 {
					targetTable = t
					break
				}
			}
		}
	}
	if targetTable == nil {
		return nil // table not found, skip filling
	}

	var fillRequests []*docs.Request
	// Iterate in reverse order so indices remain valid after inserts
	for r := len(targetTable.TableRows) - 1; r >= 0; r-- {
		row := targetTable.TableRows[r]
		for ci := len(row.TableCells) - 1; ci >= 0; ci-- {
			cell := row.TableCells[ci]
			if r >= len(spec.cells) || ci >= len(spec.cells[r]) {
				continue
			}
			cellText := spec.cells[r][ci]
			if cellText == "" {
				continue
			}
			if len(cell.Content) == 0 {
				continue
			}
			// In a table cell, the first StructuralElement is a paragraph.
			// For an empty cell, the paragraph occupies [startIndex, startIndex+1] with just a \n.
			// We insert at startIndex to place text before the trailing newline.
			insertIdx := cell.Content[0].StartIndex

			plainText, formats := parseMarkdownReplacement(cellText)

			fillRequests = append(fillRequests, &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: insertIdx},
					Text:     plainText,
				},
			})

			fillRequests = append(fillRequests, buildTextStyleRequests(formats, insertIdx, insertIdx+utf16Len(plainText))...)
		}
	}

	if len(fillRequests) > 0 {
		err = retryOnQuota(ctx, func() error {
			_, e := docsSvc.Documents.BatchUpdate(id, &docs.BatchUpdateDocumentRequest{
				Requests: fillRequests,
			}).Context(ctx).Do()
			return e
		})
		if err != nil {
			return fmt.Errorf("batch update (fill table cells): %w", err)
		}
	}
	return nil
}

func (c *DocsSedCmd) runTableCreate(ctx context.Context, u *ui.UI, account, id string, expr sedExpr, spec *tableCreateSpec) error {
	re, err := expr.compilePattern()
	if err != nil {
		return fmt.Errorf("compile pattern: %w", err)
	}

	docsSvc, doc, err := fetchDoc(ctx, account, id)
	if err != nil {
		return err
	}

	// Find the placeholder text in the document
	var matchStart, matchEnd int64
	found := false

	var walkContent func(content []*docs.StructuralElement)
	walkContent = func(content []*docs.StructuralElement) {
		if found {
			return
		}
		for _, elem := range content {
			if elem.Paragraph != nil {
				for _, pe := range elem.Paragraph.Elements {
					if pe.TextRun != nil && pe.TextRun.Content != "" {
						loc := re.FindStringIndex(pe.TextRun.Content)
						if loc != nil {
							matchStart = pe.StartIndex + int64(loc[0])
							matchEnd = pe.StartIndex + int64(loc[1])
							found = true
							return
						}
					}
				}
			}
			// Walk into table cells too
			if elem.Table != nil {
				for _, row := range elem.Table.TableRows {
					for _, cell := range row.TableCells {
						walkContent(cell.Content)
					}
				}
			}
		}
	}
	if doc.Body != nil {
		walkContent(doc.Body.Content)
	}

	if !found {
		return sedOutputOK(ctx, u, id, sedOutputKV{"replaced", 0}, sedOutputKV{"message", "pattern not found"})
	}

	// Step 1: Delete the placeholder text
	// Step 2: Insert the table at that position
	// Note: InsertTableRequest requires the location to be inside a paragraph,
	// so we insert at the start of the match.
	var requests []*docs.Request

	// Delete placeholder text
	if matchStart < matchEnd {
		requests = append(requests, &docs.Request{
			DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{
					StartIndex: matchStart,
					EndIndex:   matchEnd,
				},
			},
		})
	}

	// Insert table at the position where placeholder was
	requests = append(requests, &docs.Request{
		InsertTable: &docs.InsertTableRequest{
			Location: &docs.Location{Index: matchStart},
			Rows:     int64(spec.rows),
			Columns:  int64(spec.cols),
		},
	})

	err = retryOnQuota(ctx, func() error {
		_, e := docsSvc.Documents.BatchUpdate(id, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()
		return e
	})
	if err != nil {
		return fmt.Errorf("batch update (create table): %w", err)
	}

	// Fill cells with content if provided (pipe-table syntax)
	if len(spec.cells) > 0 {
		if err := c.fillTableCells(ctx, docsSvc, id, matchStart, spec); err != nil {
			return fmt.Errorf("fill table cells: %w", err)
		}
	}

	extra := []sedOutputKV{{"created", fmt.Sprintf("%dx%d table", spec.rows, spec.cols)}}
	if len(spec.cells) > 0 {
		extra = append(extra, sedOutputKV{"filled", true})
	}
	if spec.header {
		extra = append(extra, sedOutputKV{"header", "true (note: header pinning requires manual step in Docs UI)"})
	}
	return sedOutputOK(ctx, u, id, extra...)
}
