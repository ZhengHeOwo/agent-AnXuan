package workspace

import (
	"fmt"
	"slices"
	"strings"
)

// read_text_file_tool 结果包装
func readTextFileToolResultResponse(result *readTextFileResult) string {
	if result == nil {
		return ""
	}
	return fmt.Sprintf(
		"内容是否被截断: %t | bytes: %d | runes: %d | lines: %d\n正文:\n%s",
		result.truncated,
		result.bytes,
		result.runes,
		result.lines,
		result.content,
	)
}

// list_text_files_tool 结果包装
func listTextFilesToolResultResponse(result *textFileList) string {
	if result == nil {
		return ""
	}

	return fmt.Sprintf(
		"列表是否被截断: %t |\n列表结果:\n%q",
		result.truncated,
		result.paths,
	)
}

// write_text_file_tool 结果包装
func writeTextFileToolResultResponse(result *writeTextFileResult) string {
	if result == nil {
		return ""
	}

	if !result.written {
		return fmt.Sprintf(
			"写入 %q 被拒绝(未进行任何更改)",
			result.toolPath,
		)
	}

	return fmt.Sprintf(
		"文件 %q 写入成功",
		result.toolPath,
	)
}

// search_text_tool 结果包装
func searchTextToolResultResponse(result *TextSearchResult) string {
	if result == nil {
		return ""
	}

	var builder strings.Builder

	fmt.Fprintf(
		&builder,
		"候选列表是否被截断: %t | 搜索内容是否被截断: %t | 匹配条数: %d\n",
		result.CandidateListTruncated,
		result.SearchTruncated,
		len(result.Matches),
	)

	if len(result.Matches) != 0 {
		builder.WriteString("搜索结果:\n")
	}

	for _, match := range result.Matches {
		fmt.Fprintf(
			&builder,
			"路径: %q | 行: %d | 文本是否被截断: %t | 文本: %s\n",
			match.Path,
			match.Line,
			match.TextTruncated,
			match.Text,
		)
	}

	return builder.String()
}

// search_text_tool 问题包装
//
// 问题详情按固定类别顺序分组输出, 组内按路径排序, 保证同一批问题每次渲染结果一致。
var searchIssueKindOrder = []searchIssueKind{
	issueVerificationFailed,
	issueOpenFailed,
	issueNotRegular,
	issueFileTooLarge,
	issueLineTooLong,
	issueReadFailed,
}

func searchIssueKindLabel(kind searchIssueKind) string {
	switch kind {
	case issueVerificationFailed:
		return "不符合搜索要求"
	case issueOpenFailed:
		return "打开失败"
	case issueNotRegular:
		return "非普通文件"
	case issueFileTooLarge:
		return "文件过大未搜索"
	case issueLineTooLong:
		return "单行过长"
	case issueReadFailed:
		return "读取出错"
	}

	return string(kind)
}

// orderedSearchIssueKinds 先按固定顺序列出已知类别, 未知类别按名称排序补在末尾。
func orderedSearchIssueKinds(counts map[searchIssueKind]int) []searchIssueKind {
	kinds := make([]searchIssueKind, 0, len(counts))
	known := make(map[searchIssueKind]bool, len(searchIssueKindOrder))

	for _, kind := range searchIssueKindOrder {
		known[kind] = true
		if counts[kind] != 0 {
			kinds = append(kinds, kind)
		}
	}

	rest := make([]searchIssueKind, 0, len(counts))
	for kind := range counts {
		if !known[kind] && counts[kind] != 0 {
			rest = append(rest, kind)
		}
	}
	slices.Sort(rest)

	return append(kinds, rest...)
}

func searchIssuesOfKind(issues []searchIssue, kind searchIssueKind) []searchIssue {
	matched := make([]searchIssue, 0, len(issues))

	for _, issue := range issues {
		if issue.Kind == kind {
			matched = append(matched, issue)
		}
	}

	slices.SortFunc(matched, func(a, b searchIssue) int {
		return strings.Compare(a.Path, b.Path)
	})

	return matched
}

func searchIssueToolResultResponse(collector *issueCollector) string {
	if collector == nil || len(collector.counts) == 0 {
		return ""
	}

	total := 0
	for _, count := range collector.counts {
		total += count
	}

	kinds := orderedSearchIssueKinds(collector.counts)

	var builder strings.Builder
	fmt.Fprintf(&builder, "搜索过程中的问题: 共 %d 个\n", total)

	labels := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		labels = append(
			labels,
			fmt.Sprintf("%s %d", searchIssueKindLabel(kind), collector.counts[kind]),
		)
	}
	fmt.Fprintf(&builder, "各类数量: %s\n", strings.Join(labels, ", "))
	builder.WriteString("问题详情:\n")

	for _, kind := range kinds {
		issues := searchIssuesOfKind(collector.kept, kind)
		if len(issues) == 0 {
			continue
		}

		count := collector.counts[kind]
		if len(issues) < count {
			fmt.Fprintf(
				&builder,
				"[%s] 共 %d 个, 列出 %d 个:\n",
				searchIssueKindLabel(kind),
				count,
				len(issues),
			)
		} else {
			fmt.Fprintf(
				&builder,
				"[%s] 共 %d 个, 全部列出:\n",
				searchIssueKindLabel(kind),
				count,
			)
		}

		for _, issue := range issues {
			if issue.Err != nil {
				fmt.Fprintf(
					&builder,
					"- %q (原因: %s)\n",
					issue.Path,
					issue.Err.Error(),
				)
				continue
			}

			fmt.Fprintf(&builder, "- %q\n", issue.Path)
		}
	}

	return builder.String()
}
