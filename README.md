
![](docs/title.png)

# 背景
最近这这段时间更新了一些 k8s 相关的博客和视频，也收到了一些反馈；大概分为这几类：
- 公司已经经历过服务化改造了，但还未接触过云原生。
- 公司部分应用进行了云原生改造，但大部分工作是由基础架构和运维部门推动的，自己只是作为开发并不了解其中的细节，甚至 k8s 也接触不到。
- 还处于比较传统的以虚拟机部署的传统运维为主。

其中以第二种占大多数，虽然公司进行了云原生改造，但似乎和纯业务研发同学来说没有太大关系，自己工作也没有什么变化。

恰好我之前正好从业务研发的角度转换到了基础架构部门，两个角色我都接触过，也帮助过一些业务研发了解公司的云原生架构；

为此所以我想系统性的带大家以**研发**的角度对 k8s 进行实践。

因为 k8s 部分功能其实是偏运维的，对研发来说优先级并不太高；
所以我不太会涉及一些 k8s 运维的知识点，比如安装、组件等模块；主要以我们日常开发会使用到的组件为主。

![image.png](https://s2.loli.net/2023/09/02/BtYcF6jp8u3nzJs.png)


# 入门
- [部署应用到 k8s](#部署应用到-k8s)
- [跨服务调用](#跨服务调用)
- 集群外部访问

# 进阶
- 如何使用配置
- 服务网格实战

# 运维你的应用
- 应用探针
- 滚动更新与回滚
- 优雅采集日志
- 应用可观测性
    - 指标可视化

----

# 部署应用到 k8s

首先从第一章【部署应用到 k8s】开始，我会用 Go 写一个简单的 Web 应用，然后打包为一个 Docker 镜像，之后部署到 k8s 中，并完成其中的接口调用。

## 编写应用

```go
func main() {  
   http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {  
      log.Println("ping")  
      fmt.Fprint(w, "pong")  
   })  
  
   http.ListenAndServe(":8081", nil)  
}
```

应用非常简单就是提供了一个 `ping`  接口，然后返回了一个 `pong`.

## Dockerfile

```dockerfile
# 第一阶段：编译 Go 程序  
FROM golang:1.19 AS dependencies  
ENV GOPROXY=https://goproxy.cn,direct  
WORKDIR /go/src/app  
COPY go.mod .  
#COPY ../../go.sum .  
RUN --mount=type=ssh go mod download  
  
# 第二阶段：构建可执行文件  
FROM golang:1.19 AS builder  
WORKDIR /go/src/app  
COPY . .  
#COPY --from=dependencies /go/pkg /go/pkg  
RUN go build  
  
# 第三阶段：部署  
FROM debian:stable-slim  
RUN apt-get update && apt-get install -y curl  
COPY --from=builder /go/src/app/k8s-combat /go/bin/k8s-combat  
ENV PATH="/go/bin:${PATH}"  
  
# 启动 Go 程序  
CMD ["k8s-combat"]
```

之后编写了一个 `dockerfile` 用于构建 `docker` 镜像。

```makefile
docker:  
   @echo "Docker Build..."  
   docker build . -t crossoverjie/k8s-combat:v1 && docker image push crossoverjie/k8s-combat:v1
```

使用 `make docker`  会在本地构建镜像并上传到 `dockerhub`

## 编写 deployment
下一步便是整个过程中最重要的环节了，也是唯一和 k8s 打交道的地方，那就是编写 deployment。


在之前的视频[《一分钟了解 k8s》](【一分钟带你了解 k8s】 https://www.bilibili.com/video/BV1Cm4y1n7yG/?share_source=copy_web&vd_source=358858ab808efe832b0dda9dbc4701da)
中讲过常见的组件：
![image.png](https://s2.loli.net/2023/09/04/hrOUSVsmP2KkNlC.png)

其中我们最常见的就是 deployment，通常用于部署无状态应用；现在还不太需要了解其他的组件，先看看 deployment 如何编写：
```yaml
apiVersion: apps/v1  
kind: Deployment  
metadata:  
  labels:  
    app: k8s-combat  
  name: k8s-combat  
spec:  
  replicas: 1  
  selector:  
    matchLabels:  
      app: k8s-combat  
  template:  
    metadata:  
      labels:  
        app: k8s-combat  
    spec:  
      containers:  
        - name: k8s-combat  
          image: crossoverjie/k8s-combat:v1  
          imagePullPolicy: Always  
          resources:  
            limits:  
              cpu: "1"  
              memory: 300Mi  
            requests:  
              cpu: "0.1"  
              memory: 30Mi
```

开头两行的 `apiVersion`  和 `kind` 可以暂时不要关注，就理解为 deployment 的固定写法即可。

metadata：顾名思义就是定义元数据的地方，告诉 `Pod` 我们这个 `deployment` 叫什么名字，这里定义为：`k8s-combat`

中间的：
```yaml
metadata:  
  labels:  
    app: k8s-combat
```

也很容易理解，就是给这个 `deployment` 打上标签，通常是将这个标签和其他的组件进行关联使用才有意义，不然就只是一个标签而已。
> 标签是键值对的格式，key, value 都可以自定义。

而这里的  `app: k8s-combat` 便是和下面的 spec 下的 selector 选择器匹配，表明都使用  `app: k8s-combat`  进行关联。

而 template 中所定义的标签也是为了让选择器和 template 中的定义的 Pod 进行关联。

> Pod 是 k8s 中相同功能容器的分组，一个 Pod 可以绑定多个容器，这里就只有我们应用容器一个了；后续在讲到 istio 和日志采集时便可以看到其他的容器。

template 中定义的内容就很容易理解了，指定了我们的容器拉取地址，以及所占用的资源(`cpu/ memory`)。

`replicas: 1`：表示只部署一个副本，也就是只有一个节点的意思。

## 部署应用

之后我们使用命令:

```shell
kubectl apply -f deployment/deployment.yaml
```

> 生产环境中往往会使用云厂商所提供的 k8s 环境，我们本地可以使用 [https://minikube.sigs.k8s.io/docs/start/](https://minikube.sigs.k8s.io/docs/start/) minikube 来模拟。

就会应用这个 deployment 同时将容器部署到 k8s 中，之后使用:
```shell
kubectl get pod
```
>  在后台 k8s 会根据我们填写的资源选择一个合适的节点，将当前这个 Pod 部署过去。

就会列出我们刚才部署的 Pod:
```shell
❯ kubectl get pod
NAME                                READY   STATUS    RESTARTS   AGE
k8s-combat-57f794c59b-7k58n         1/1     Running   0          17h
```

我们使用命令：
```shell
kubectl exec -it k8s-combat-57f794c59b-7k58n  bash
```
就会进入我们的容器，这个和使用 docker 类似。

之后执行 curl 命令便可以访问我们的接口了：
```shell
root@k8s-combat-57f794c59b-7k58n:/# curl http://127.0.0.1:8081/ping
pong
root@k8s-combat-57f794c59b-7k58n:/#
```

这时候我们再开一个终端执行：
```
❯ kubectl logs -f k8s-combat-57f794c59b-7k58n
2023/09/03 09:28:07 ping
```
便可以打印容器中的日志，当然前提是应用的日志是写入到了标准输出中。




# 跨服务调用

在做传统业务开发的时候，当我们的服务提供方有多个实例时，往往我们需要将对方的服务列表保存在本地，然后采用一定的算法进行调用；当服务提供方的列表变化时还得及时通知调用方。

```yaml
student:
	url:
		- 192.168.1.1:8081
		- 192.168.1.2:8081
```

这样自然是对双方都带来不少的负担，所以后续推出的服务调用框架都会想办法解决这个问题。

以 `spring cloud` 为例：
![image.png](https://s2.loli.net/2023/09/06/IW1jaidQ25Xk9u4.png)

服务提供方会向一个服务注册中心注册自己的服务（名称、IP等信息），客户端每次调用的时候会向服务注册中心获取一个节点信息，然后发起调用。

但当我们切换到 `k8s` 后，这些基础设施都交给了 `k8s` 处理了，所以 `k8s` 自然得有一个组件来解决服务注册和调用的问题。

也就是我们今天重点介绍的 `service`。


# service

在介绍 `service` 之前我先调整了源码：
```go
func main() {  
   http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {  
      name, _ := os.Hostname()  
      log.Printf("%s ping", name)  
      fmt.Fprint(w, "pong")  
   })  
   http.HandleFunc("/service", func(w http.ResponseWriter, r *http.Request) {  
      resp, err := http.Get("http://k8s-combat-service:8081/ping")  
      if err != nil {  
         log.Println(err)  
         fmt.Fprint(w, err)  
         return  
      }  
      fmt.Fprint(w, resp.Status)  
   })  
  
   http.ListenAndServe(":8081", nil)  
}
```
新增了一个 `/service` 的接口，这个接口会通过 service 的方式调用服务提供者的服务，然后重新打包。

```shell
make docker
```

同时也新增了一个 `deployment-service.yaml`:
```yaml
apiVersion: apps/v1  
kind: Deployment  
metadata:  
  labels:  
    app: k8s-combat-service # 通过标签选择关联  
  name: k8s-combat-service  
spec:  
  replicas: 1  
  selector:  
    matchLabels:  
      app: k8s-combat-service  
  template:  
    metadata:  
      labels:  
        app: k8s-combat-service  
    spec:  
      containers:  
        - name: k8s-combat-service  
          image: crossoverjie/k8s-combat:v1  
          imagePullPolicy: Always  
          resources:  
            limits:  
              cpu: "1"  
              memory: 100Mi  
            requests:  
              cpu: "0.1"  
              memory: 10Mi  
---  
apiVersion: v1  
kind: Service  
metadata:  
  name: k8s-combat-service  
spec:  
  selector:  
    app: k8s-combat-service # 通过标签选择关联  
  type: ClusterIP  
  ports:  
    - port: 8081        # 本 Service 的端口  
      targetPort: 8081  # 容器端口  
      name: app
```

使用相同的镜像部署一个新的 deployment，名称为 `k8s-combat-service`，重点是新增了一个`kind: Service` 的对象。

这个就是用于声明 `service` 的组件，在这个组件中也是使用 `selector` 标签和 `deployment` 进行了关联。

也就是说这个 `service` 用于服务于名称等于 `k8s-combat-service` 的 `deployment`。

下面的两个端口也很好理解，一个是代理的端口， 另一个是  service 自身提供出去的端口。

至于 `type: ClusterIP` 是用于声明不同类型的 `service`，除此之外的类型还有：
- [`NodePort`](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport)
- [`LoadBalancer`](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer)
- [`ExternalName`](https://kubernetes.io/docs/concepts/services-networking/service/#externalname)
  等类型，默认是 `ClusterIP`，现在不用纠结这几种类型的作用，后续我们在讲到 `Ingress` 的时候会具体介绍。

## 负载测试
我们先分别将这两个 `deployment` 部署好：
```shell
k apply -f deployment/deployment.yaml
k apply -f deployment/deployment-service.yaml

❯ k get pod
NAME                                  READY   STATUS    RESTARTS   AGE
k8s-combat-7867bfb596-67p5m           1/1     Running   0          3h22m
k8s-combat-service-5b77f59bf7-zpqwt   1/1     Running   0          3h22m
```

由于我新增了一个 `/service` 的接口，用于在 `k8s-combat` 中通过 `service` 调用 `k8s-combat-service` 的接口。

```go
resp, err := http.Get("http://k8s-combat-service:8081/ping")
```

其中 `k8s-combat-service` 服务的域名就是他的服务名称。
> 如果是跨 namespace 调用时，需要指定一个完整名称，在后续的章节会演示。



我们整个的调用流程如下：
![image.png](https://s2.loli.net/2023/09/06/i12pR3DjC6wnIXQ.png)

相信大家也看得出来相对于 `spring cloud` 这类微服务框架提供的客户端负载方式，`service` 是一种服务端负载，有点类似于 `Nginx` 的反向代理。

为了更直观的验证这个流程，此时我将 `k8s-combat-service` 的副本数增加到 2：
```yaml
spec:  
  replicas: 2
```

只需要再次执行：
```shell
❯ k apply -f deployment/deployment-service.yaml
deployment.apps/k8s-combat-service configured
service/k8s-combat-service unchanged
```

![image.png](https://s2.loli.net/2023/09/06/ZC8UrjEz6ia1Qgo.png)

> 不管我们对 `deployment` 的做了什么变更，都只需要 `apply` 这个 `yaml`  文件即可， k8s 会自动将当前的 `deployment` 调整为我们预期的状态（比如这里的副本数量增加为 2）；这也就是 `k8s` 中常说的**声明式 API**。


可以看到此时 `k8s-combat-service` 的副本数已经变为两个了。
如果我们此时查看这个 `service` 的描述时：

```shell
❯ k describe svc k8s-combat-service |grep Endpoints
Endpoints:         192.168.130.133:8081,192.168.130.29:8081
```
会发现它已经代理了这两个 `Pod` 的 IP。


![image.png](https://s2.loli.net/2023/09/06/HbjyEcnaeCK6uMJ.png)
此时我进入了 `k8s-combat-7867bfb596-67p5m` 的容器：

```shell
k exec -it k8s-combat-7867bfb596-67p5m bash
curl http://127.0.0./service
```

并执行两次 `/service` 接口，发现请求会轮训进入 `k8s-combat-service` 的代理的 IP 中。

由于 `k8s service` 是基于 `TCP/UDP` 的四层负载，所以在 `http1.1`  中是可以做到请求级的负载均衡，但如果是类似于 `gRPC` 这类长链接就无法做到请求级的负载均衡。

换句话说 `service` 只支持连接级别的负载。

如果要支持 `gRPC`，就得使用 Istio 这类服务网格，相关内容会在后续章节详解。

总的来说 `k8s service` 提供了简易的服务注册发现和负载均衡功能，当我们只提供 http 服务时是完全够用的。