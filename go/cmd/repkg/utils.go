// 本文件提供 extract/info 命令共用的辅助函数。
package main

import (
	"slices"
	"strings"
)

// getSafeFilename 替换非法文件名字符为下划线。
func getSafeFilename(name string) string {
	if name == "" {
		return ""
	}
	invalid := `/<>:"|?*` + "\x00"
	result := []rune(name)
	for i, c := range result {
		if strings.ContainsRune(invalid, c) {
			result[i] = '_'
		}
	}
	return string(result)
}

// getPropertyKeys 返回 JSON 对象的所有键名（按字母排序）。
func getPropertyKeys(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// containsString 判断 haystack 中是否包含 needle（忽略大小写）。
func containsString(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
