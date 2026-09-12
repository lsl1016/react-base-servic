package mcpclient

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// resolveHost 可注入的域名解析函数（测试替换用），生产环境为 net.LookupIP。
var resolveHost = func(host string) []net.IP {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil
	}
	return ips
}

// reservedPrefixes 是额外拒绝的保留网段（文档/基准/CGNAT/基准测试网），
// netip 的 IsPrivate/IsLoopback 未覆盖的部分在此补齐。
var reservedPrefixes = []netip.Prefix{
	mustPrefix("100.64.0.0/10"),   // CGNAT
	mustPrefix("192.0.0.0/24"),    // IETF 协议分配
	mustPrefix("192.0.2.0/24"),    // TEST-NET-1
	mustPrefix("198.18.0.0/15"),   // 基准测试
	mustPrefix("198.51.100.0/24"), // TEST-NET-2
	mustPrefix("203.0.113.0/24"),  // TEST-NET-3
	mustPrefix("240.0.0.0/4"),     // 保留段
	mustPrefix("fc00::/7"),        // IPv6 ULA
	mustPrefix("2001:db8::/32"),   // IPv6 文档前缀
}

func mustPrefix(cidr string) netip.Prefix {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		panic(err)
	}
	return prefix
}

// isPublicAddr 判断 IP 是否为可对外访问的地址：
// 拒绝环回、私网、链路本地、组播、未指定、保留与文档网段。
func isPublicAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	if addr.Zone() != "" {
		return false
	}
	for _, prefix := range reservedPrefixes {
		if prefix.Contains(addr.Unmap()) {
			return false
		}
	}
	return true
}

// validateEndpoint 校验 MCP HTTP 服务端点（SSRF 防护，每次建会话前执行）：
// 仅允许 http/https；主机名为 IP 字面量时直接判定，为域名时解析后逐一判定，
// 任一解析结果落入禁止网段即拒绝（缓解 DNS 重绑定）。
func validateEndpoint(raw string) (*url.URL, error) {
	return validateEndpointOpts(raw, false)
}

// validateEndpointOpts 是 validateEndpoint 的可选项版本：
// allowPrivate=true 时放行环回/私网/保留地址，仅供本机开发/演示环境
// （mcp.allow_private_endpoint 配置开关）显式开启；其余行为一致。
func validateEndpointOpts(raw string, allowPrivate bool) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("mcp endpoint is empty")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("mcp endpoint parse: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("mcp endpoint scheme must be http/https, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("mcp endpoint missing host")
	}

	if addr, parseErr := netip.ParseAddr(host); parseErr == nil {
		if !allowPrivate && !isPublicAddr(addr.Unmap()) {
			return nil, fmt.Errorf("mcp endpoint host %s is loopback/private/reserved", host)
		}
		return u, nil
	}

	ips := resolveHost(host)
	if len(ips) == 0 {
		return nil, fmt.Errorf("mcp endpoint host %s cannot be resolved", host)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		if !allowPrivate && !isPublicAddr(addr.Unmap()) {
			return nil, fmt.Errorf("mcp endpoint host %s resolves to non-public address %s", host, ip)
		}
	}
	return u, nil
}
