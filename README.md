# weiss
一个go lib，允许通过本地proxy的方式直连pixiv  

# compile
如果是go可以直接引用，gomod已经写好  
需要使用`goproxy`提供的证书生成方式生成自己的证书
```
cd ./goproxy/certs/
bash openssl-gen.sh
```
使用
```
weiss.Start("7890")
weiss.Stop()
``` 
# 缺陷  
`Doh`方式获取真实ip仍然存在cloudflare套壳的问题，需要及时硬编码更新  
可以修改`onezero.go`的`hardcodeIpMap`达成硬编码的目的  
黑名单虽然可以加速超时，但是会影响人机验证 

如果有什么更好的方法，欢迎交流
