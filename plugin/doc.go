// Package plugin 提供编译期插件注册中心，供扩展注册模型、工具、中间件与事件监听器。
//
// 插件通过匿名导入触发 init() 注册，例如：
//
//	import _ "your/plugin/path"
//
// 当前阶段不支持运行时热插拔；所有注册应在进程启动完成前完成。

package plugin
