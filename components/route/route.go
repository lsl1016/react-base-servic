package route

import "encoding/json"

// BuildRoutePrefixes 根据请求的 routeValues 生成所有前缀路径，用于 IN 查询
// 例如 ["space_abc", "rpt_123"] → ["[]", '["space_abc"]', '["space_abc","rpt_123"]']
func BuildRoutePrefixes(routeValues []string) []string {
	prefixes := []string{"[]"}
	for i := 1; i <= len(routeValues); i++ {
		b, _ := json.Marshal(routeValues[:i])
		prefixes = append(prefixes, string(b))
	}
	return prefixes
}

// SplitRouteExactAndParents 将 routeValues 拆分为精确匹配值和上级前缀列表
// 例如 ["s_xxx", "r_xxx"] → exact: '["s_xxx","r_xxx"]', parents: ['[]', '["s_xxx"]']
// 例如 ["s_xxx"] → exact: '["s_xxx"]', parents: ['[]']
// 例如 [] → exact: '[]', parents: []
func SplitRouteExactAndParents(routeValues []string) (exact string, parents []string) {
	if routeValues == nil {
		routeValues = []string{}
	}
	b, _ := json.Marshal(routeValues)
	exact = string(b)

	parents = []string{}
	if exact == "[]" {
		return exact, parents
	}

	parents = append(parents, "[]")
	for i := 1; i < len(routeValues); i++ {
		p, _ := json.Marshal(routeValues[:i])
		parents = append(parents, string(p))
	}
	return exact, parents
}

// SerializeRouteValues 将 routeValues 序列化为 JSON 字符串，用于精确匹配查询
func SerializeRouteValues(routeValues []string) string {
	if routeValues == nil {
		routeValues = []string{}
	}
	b, _ := json.Marshal(routeValues)
	return string(b)
}
