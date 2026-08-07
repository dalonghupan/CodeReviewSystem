package mq

import "net"

// resolveNameServers 将 namesrv 地址中的主机名解析为 IP
// rocketmq-client-go v2 的 NamesrvAddr.Check() 仅接受 IP:port，主机名（如 K8s/Docker
// 服务名）会被拒绝（IP addr error），因此在创建客户端前做一次 DNS 解析。
// 解析失败的地址保持原值，错误交由客户端连接阶段上报。
func resolveNameServers(addrs []string) []string {
	resolved := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || net.ParseIP(host) != nil {
			resolved = append(resolved, addr)
			continue
		}
		ips, err := net.LookupHost(host)
		if err != nil || len(ips) == 0 {
			resolved = append(resolved, addr)
			continue
		}
		resolved = append(resolved, net.JoinHostPort(ips[0], port))
	}
	return resolved
}
