# oraprofiler
a simple realtime oracle sql profiler by watching *.trc files

Oracle Profiler, 只有一个二进制执行文件, 通过监视 .trc 文件实时抓取 SQL 语句, 本程序需要连接数据库读取 session 信息并开关 tracing 功能, 无其它危险行为.
(兼容 Oracle 10g+)

```
Usage: oraprofiler -conn=CONNECTION_STRING [-addr=[IP]:PORT]
  -addr address
    	Web UI address (default ":3456")
  -conn connection string
    	Oracle database connection string
  -help
    	Show help
```
