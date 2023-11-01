
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
- [集群外部访问-Ingress](#集群外部访问)

# 进阶
- [如何使用配置](#如何使用配置)
- [服务网格实战](#istio-入门)

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
curl http://127.0.0.1:8081/service
```

并执行两次 `/service` 接口，发现请求会轮训进入 `k8s-combat-service` 的代理的 IP 中。

由于 `k8s service` 是基于 `TCP/UDP` 的四层负载，所以在 `http1.1`  中是可以做到请求级的负载均衡，但如果是类似于 `gRPC` 这类长链接就无法做到请求级的负载均衡。

换句话说 `service` 只支持连接级别的负载。

如果要支持 `gRPC`，就得使用 Istio 这类服务网格，相关内容会在后续章节详解。

总的来说 `k8s service` 提供了简易的服务注册发现和负载均衡功能，当我们只提供 http 服务时是完全够用的。


# 集群外部访问

前两章中我们将应用[部署](https://crossoverjie.top/2023/08/31/ob/k8s-0-start/)到了 k8s 中，同时不同的服务之间也可以通过 [service](https://crossoverjie.top/2023/09/05/ob/k8s-service/) 进行调用，现在还有一个步骤就是将我们的应用暴露到公网，并提供域名的访问。

这一步类似于我们以前配置 Nginx 和绑定域名，提供这个能力的服务在 k8s 中成为 Ingress。

通过这个描述其实也能看出 Ingress 是偏运维的工作，但也不妨碍我们作为研发去了解这部分的内容；了解整个系统是如何运转的也是研发应该掌握的技能。

# 安装 Ingress 控制器
在正式使用 Ingress 之前需要给 k8s 安装一个 Ingress 控制器，我们这里安装官方提供的 Ingress-nginx 控制器。

当然还有社区或者企业提供的各种控制器：
![image.png](https://s2.loli.net/2023/09/14/i1ebXQNUjxPkLEZ.png)


有两种安装方式: helm 或者是直接 apply 一个资源文件。

关于 `helm` 我们会在后面的章节单独讲解。

这里就直接使用资源文件安装即可，我已经上传到 GitHub 可以在这里访问：
[https://github.com/crossoverJie/k8s-combat/blob/main/deployment/ingress-nginx.yaml](https://github.com/crossoverJie/k8s-combat/blob/main/deployment/ingress-nginx.yaml)

其实这个文件也是直接从官方提供的复制过来的，也可以直接使用这个路径进行安装：
```yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
```

> yaml 文件的内容是一样的。

不过要注意安装之后可能容器状态一直处于 Pending 状态，查看容器的事件时会发现镜像拉取失败。

```shell
k describe pod ingress-nginx-controller-7cdfb9988c-lbcst -n ingress-nginx
```

> describe 是一个用于查看 k8s 对象详细信息的命令。

在刚才那份 yaml 文件中可以看到有几个镜像需要拉取，我们可以先在本地手动拉取镜像：
![image.png](https://s2.loli.net/2023/09/14/3IsRe2QWcmjTY41.png)
```shell
docker pull registry.k8s.io/ingress-nginx/controller:v1.8.2
```

如果依然无法拉取，可以尝试配置几个国内镜像源镜像拉取：

![image.png](https://s2.loli.net/2023/09/14/uTNDACSWdPp7BVt.png)

> 我这里使用的 docker-desktop 自带的 k8s，推荐读者朋友也使用这个工具。

# 创建 Ingress
使用刚才的 yaml 安装成功之后会在 `ingress-nginx` 命名空间下创建一个 Pod，通过 get 命令查看状态为 Running 即为安装成功。
```shell
$ k get pod -n ingress-nginx
NAME                            READY   STATUS    RESTARTS      AGE
ingress-nginx-controller-7cdf   1/1     Running   2 (35h ago)   3d
```

> Namespace 也是 k8s 内置的一个对象，可以简单理解为对资源进行分组管理，我们通常可以使用它来区分各个不同的环境，比如 dev/test/prod 等，不同命名空间下的资源不会互相干扰，且相互独立。


之后便可以创建 Ingress 资源了：
```yaml
apiVersion: networking.k8s.io/v1  
kind: Ingress  
metadata:  
  name: k8s-combat-ingress  
spec:  
  ingressClassName: nginx  
  rules:  
    - host: www.service1.io  
      http:  
        paths:  
          - backend:  
              service:  
                name: k8s-combat-service  
                port:  
                  number: 8081  
            path: /  
            pathType: Prefix  
    - host: www.service2.io  
      http:  
        paths:  
          - backend:  
              service:  
                name: k8s-combat-service-2  
                port:  
                  number: 8081  
            path: /  
            pathType: Prefix
```

看这个内容也很容易理解，创建了一个 `Ingress` 的对象，其中的重点就是这里的规则是如何定义的。

> 在 k8s 中今后还会接触到各种不同的 Kind

这里的 `ingressClassName: nginx`   也是在刚开始安装的控制器里定义的名字，由这个资源定义。

```yaml
apiVersion: networking.k8s.io/v1  
kind: IngressClass  
metadata:  
  labels:  
    app.kubernetes.io/component: controller  
    app.kubernetes.io/instance: ingress-nginx  
    app.kubernetes.io/name: ingress-nginx  
    app.kubernetes.io/part-of: ingress-nginx  
    app.kubernetes.io/version: 1.8.2  
  name: nginx
```

咱们这个规则很简单，就是将两个不同的域名路由到两个不同的 service。

> 这里为了方便测试又创建了一个 `k8s-combat-service-2` 的 service，和 `k8s-combat-service` 是一样的，只是改了个名字而已。

# 测试
也是为了方便测试，我在应用镜像中新增了一个接口，用于返回当前 Pod 的 hostname。
```go
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {  
   name, _ := os.Hostname()  
   fmt.Fprint(w, name)  
})
```


由于我实际并没有 `www.service1.io/www.service2.io` 这两个域名，所以只能在本地配置 host 进行模拟。

```
10.0.0.37 www.service1.io
10.0.0.37 www.service2.io
```

> 我测试所使用的 k8s 部署在我家里一台限制的 Mac 上，所以这里的 IP 它的地址。


当我们反复请求两次这个接口，会拿到两个不同的 hostname，也就是将我们的请求轮训负载到了这两个 service 所代理的两个 Pod 中。

```shell
❯ curl http://www.service1.io/
k8s-combat-service-79c5579587-b6nlj%
❯ curl http://www.service1.io/
k8s-combat-service-79c5579587-bk7nw%
❯ curl http://www.service2.io/
k8s-combat-service-2-7bbf56b4d9-dkj9b%
❯ curl http://www.service2.io/
k8s-combat-service-2-7bbf56b4d9-t5l4g
```

我们也可以直接使用 describe 查看我们的 ingress 定义以及路由规则：
![image.png](https://s2.loli.net/2023/09/14/pgZzVb1L4aQTMwn.png)

```shell
$ k describe ingress k8s-combat-ingress
Name:             k8s-combat-ingress
Labels:           <none>
Namespace:        default
Address:          localhost
Ingress Class:    nginx
Default backend:  <default>
Rules:
  Host             Path  Backends
  ----             ----  --------
  www.service1.io
                   /   k8s-combat-service:8081 (10.1.0.65:8081,10.1.0.67:8081)
  www.service2.io
                   /   k8s-combat-service-2:8081 (10.1.0.63:8081,10.1.0.64:8081)
Annotations:       <none>
Events:            <none>
```

如果我们手动新增一个域名解析：
```shell
10.0.0.37 www.service3.io
❯ curl http://www.service3.io/
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>nginx</center>
</body>
</html>
```
会直接 404，这是因为没有找到这个域名的规则。

# 访问原理
![image.png](https://s2.loli.net/2023/09/14/9JTfp6GP24VmzAK.png)
整个的请求路径如上图所示，其实我们的 Ingress 本质上也是一个 service（所以它也可以启动多个副本来进行负载），只是他的类型是 `LoadBalancer`，通常这种类型的 service 会由云厂商绑定一个外部 IP，这样就可以通过这个外部 IP 访问 Ingress 了。

> 而我们应用的 service 是 ClusterIP，只能在应用内部访问

![image.png](https://s2.loli.net/2023/09/14/Bu67SlMLak1hirc.png)

通过 service 的信息也可以看到，我们 ingress 的 service 绑定的外部 IP 是 `localhost`（本地的原因）

# 总结
Ingress 通常是充当网关的作用，后续我们在使用 Istio 时，也可以使用 Istio 所提供的控制器来替换掉 Ingress-nginx，可以更方便的管理内外网流量。

本文的所有源码在这里可以访问：
[https://github.com/crossoverJie/k8s-combat](https://github.com/crossoverJie/k8s-combat)


# 如何使用配置
在前面[三节中](https://crossoverjie.top/categories/k8s/)已经讲到如何将我们的应用部署到 k8s 集群并提供对外访问的能力，x现在可以满足基本的应用开发需求了。

现在我们需要更进一步，使用 k8s 提供的一些其他对象来标准化我的应用开发。
首先就是 `ConfigMap`，从它的名字也可以看出这是用于管理配置的对象。

# ConfigMap

不管我们之前是做 `Java`、`Go` 还是 `Python` 开发都会使用到配置文件，而 `ConfigMap` 的作用可以将我们原本写在配置文件里的内容转存到 `k8s` 中，然后和我们的 `Container` 进行绑定。

## 存储到环境变量
绑定的第一种方式就是将配置直接写入到环境变量，这里我先定义一个 `ConfigMap`：
```yaml
apiVersion: v1  
kind: ConfigMap  
metadata:  
  name: k8s-combat-configmap  
data:  
  PG_URL: "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
```

重点是 `data` 部分，存储的是一个 `KV` 结构的数据，这里存储的是一个数据库连接。
> 需要注意，KV 的大小不能超过 1MB

接着可以在容器定义中绑定这个 `ConfigMap` 的所有 `KV` 到容器的环境变量：
```yaml
# Define all the ConfigMap's data as container environment variables 
envFrom:  
  - configMapRef:  
      name: k8s-combat-configmap
```

我将 `ConfigMap` 的定义也放在了同一个 [deployment](https://github.com/crossoverJie/k8s-combat/blob/main/deployment/deployment.yaml) 中，直接 apply:
```shell
❯ k apply -f deployment/deployment.yaml
deployment.apps/k8s-combat created
configmap/k8s-combat-configmap created
```

此时 `ConfigMap` 也会被创建，我们可以使用
```shell
❯ k get configmap
NAME                   DATA   AGE
k8s-combat-configmap   1      3m17s

❯ k describe configmap k8s-combat-configmap
Data
====
PG_URL:
----
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```
拿到刚才声明的配置信息。

----
同时我在代码中也读取了这个环境变量：

```go
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {  
   name, _ := os.Hostname()  
   url := os.Getenv("PG_URL")   
   fmt.Fprint(w, fmt.Sprintf("%s-%s", name, url))  
})
```

访问这个接口便能拿到这个环境变量：
```shell
root@k8s-combat-7b987bb496-pqt9s:/# curl http://127.0.0.1:8081
k8s-combat-7b987bb496-pqt9s-postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable

root@k8s-combat-7b987bb496-pqt9s:/# echo $PG_URL
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

## 存储到文件
有些时候我们也需要将这些配置存储到一个文件中，比如在 Java 中可以使用 `spring` 读取，`Go` 也可以使用 `configor` 这些第三方库来读取，所有配置都在一个文件中也更方便维护。

![image.png](https://s2.loli.net/2023/09/26/g2IhktH7iwWb8LT.png)
在 `ConfigMap` 中新增了一个 `key:APP` 存放了一个 `yaml` 格式的数据，然后在容器中使用 `volumes` 和 `volumeMounts` 将数据挂载到容器中的指定路径`/go/bin/app.yaml`

apply 之后我们可以在容器中查看这个文件是否存在：
```shell
root@k8s-combat-7b987bb496-pqt9s:/# cat /go/bin/app.yaml
name: k8s-combat
pulsar:
  url: "pulsar://localhost:6650"
  token: "abc"
```
配置已经成功挂载到了这个路径，我们便可以在代码中读取这些数据。

# Secret
可以看到 `ConfigMap` 中是明文存储数据的；
```shell
k describe configmap k8s-combat-configmap
```
可以直接查看。

对一些敏感数据就不够用了，这时我们可以使用 `Secret`:
```yaml
apiVersion: v1  
kind: Secret  
metadata:  
  name: k8s-combat-secret  
type: Opaque  
data:  
  PWD: YWJjCg==

---
env:  
  - name: PG_PWD  
    valueFrom:  
      secretKeyRef:  
        name: k8s-combat-secret  
        key: PWD
```

这里我新增了一个 `Secret` 用于存储密码，并在 `container` 中也将这个 `key` 写入到环境变量中。

```shell
❯ echo 'abc' | base64
YWJjCg==
```
`Secret` 中的数据需要使用 `base64` 进行编码，所以我这里存储的是 abc.

apply 之后我们再查看这个 `Secret` 是不能直接查看原始数据的。
```shell
❯ k describe secret k8s-combat-secret
Name:         k8s-combat-secret
Type:  Opaque

Data
====
PWD:  4 bytes
```

`Secret` 相比 `ConfigMap` 多了一个 `Type` 选项。
![](https://s2.loli.net/2023/09/26/G25TRcSzCbIVDQ3.png)

我们现阶段在应用中用的最多的就是这里的 `Opaque`，其他的暂时还用不上。


# 总结


在实际开发过程中研发人员基本上是不会直接接触 `ConfigMap`，一般会给开发者在管理台提供维护配置的页面进行 CRUD。

由于 `ConfigMap` 依赖于 k8s 与我们应用的语言无关，所以一些高级特性，比如实时更新就无法实现，每次修改后都得重启应用才能生效。

类似于 Java 中常见的配置中心：`Apollo,Nacos` 使用上会有不小的区别，但这些是应用语言强绑定的，如果业务对这些配置中心特性有强烈需求的话也是可以使用的。

但如果团队本身就是多语言研发，想要降低运维复杂度 `ConfigMap` 还是不二的选择。


# Istio 入门
终于进入大家都比较感兴趣的服务网格系列了，在前面已经讲解了：
- 如何部署应用到 `kubernetes`
- 服务之间如何调用
- 如何通过域名访问我们的服务
- 如何使用 `kubernetes` 自带的配置 `ConfigMap`

基本上已经够我们开发一般规模的 web 应用了；但在企业中往往有着复杂的应用调用关系，应用与应用之间的请求也需要进行管理。
比如常见的限流、降级、trace、监控、负载均衡等功能。

在我们使用 `kubernetes` 之前往往都是由微服务框架来解决这些问题，比如 Dubbo、SpringCloud 都有对应的功能。

但当我们上了 `kubernetes` 之后这些事情就应该交给一个专门的云原生组件来解决，也就是本次会讲到的 `Istio`，它是目前使用最为广泛的服务网格解决方案。

<!--more-->
![image.png](https://s2.loli.net/2023/10/31/CtJsogSyPD7cjEW.png)
官方对于 Istio 的解释比较简洁，落到具体的功能点也就是刚才提到的：
- 限流降级
- 路由转发、负载均衡
- 入口网关、`TLS安全认证`
- 灰度发布等

![image.png](https://s2.loli.net/2023/10/31/aXnNZhu91m7V2Tw.png)

再结合官方的架构图可知：Istio 分为控制面 `control plane` 和数据面 `data plane`。

控制面可以理解为 Istio 自身的管理功能：
- 比如服务注册发现
- 管理配置数据面所需要的网络规则等

而数据面可以简单的把他理解为由 `Envoy` 代理的我们的业务应用，我们应用中所有的流量进出都会经过 `Envoy` 代理。

所以它可以实现负载均衡、熔断保护、认证授权等功能。
# 安装
首先安装 Istio 命令行工具
> 这里的前提是有一个 kubernetes 运行环境

Linux 使用：
```shell
curl -L https://istio.io/downloadIstio | sh -
```

Mac 可以使用 brew：
```shell
brew install istioctl
```

其他环境可以下载 Istio 后配置环境变量：
```shell
export PATH=$PWD/bin:$PATH
```

之后我们可以使用 `install` 命令安装控制面。
> 这里默认使用的是 `kubectl` 所配置的 `kubernetes` 集群
```bash
istioctl install --set profile=demo -y
```
![](https://s2.loli.net/2023/10/30/DLOeRGrA7gNC1Xa.png)
这个的 `profile` 还有以下不同的值，为了演示我们使用 `demo` 即可。
![image.png](https://s2.loli.net/2023/10/26/3JXneYvyqI4WTgt.png)
# 使用
```bash
# 开启 default 命名空间自动注入
$ k label namespace default istio-injection=enabled

$ k describe ns default
Name:         default
Labels:       istio-injection=enabled
              kubernetes.io/metadata.name=default
Annotations:  <none>
Status:       Active
No resource quota.
No LimitRange resource.
```
之后我们为 `namespace` 打上 `label`，使得 Istio 控制面知道哪个 `namespace` 下的 `Pod` 会自动注入 `sidecar`。

这里我们为 default 这个命名空间打开自动注入 `sidecar`，然后在这里部署我们之前使用到的 [deployment-istio.yaml](https://github.com/crossoverJie/k8s-combat/blob/main/deployment/deployment-istio.yaml)
```bash
$ k apply -f deployment/deployment-istio.yaml

$ k get pod
NAME                                  READY   STATUS    RESTARTS
k8s-combat-service-5bfd78856f-8zjjf   2/2     Running   0          
k8s-combat-service-5bfd78856f-mblqd   2/2     Running   0          
k8s-combat-service-5bfd78856f-wlc8z   2/2     Running   0       
```
此时会看到每个Pod 有两个 container（其中一个就是 istio-proxy sidecar），也就是之前做 [gRPC 负载均衡](https://crossoverjie.top/2023/10/16/ob/k8s-grpc-lb/)测试时的代码。

![image.png](https://s2.loli.net/2023/10/31/js1Gz5yVCNLep9W.png)
还是进行负载均衡测试，效果是一样的，说明 `Istio` 起作用了。

此时我们再观察 `sidecar` 的日志时，会看到刚才我们所发出和接受到的流量：
```bash
$ k logs -f k8s-combat-service-5bfd78856f-wlc8z -c istio-proxy

[2023-10-31T14:52:14.279Z] "POST /helloworld.Greeter/SayHello HTTP/2" 200 - via_upstream - "-" 12 61 14 9 "-" "grpc-go/1.58.3" "6d293d32-af96-9f87-a8e4-6665632f7236" "k8s-combat-service:50051" "172.17.0.9:50051" inbound|50051|| 127.0.0.6:42051 172.17.0.9:50051 172.17.0.9:40804 outbound_.50051_._.k8s-combat-service.default.svc.cluster.local default
[2023-10-31T14:52:14.246Z] "POST /helloworld.Greeter/SayHello HTTP/2" 200 - via_upstream - "-" 12 61 58 39 "-" "grpc-go/1.58.3" "6d293d32-af96-9f87-a8e4-6665632f7236" "k8s-combat-service:50051" "172.17.0.9:50051" outbound|50051||k8s-combat-service.default.svc.cluster.local 172.17.0.9:40804 10.101.204.13:50051 172.17.0.9:54012 - default
[2023-10-31T14:52:15.659Z] "POST /helloworld.Greeter/SayHello HTTP/2" 200 - via_upstream - "-" 12 61 35 34 "-" "grpc-go/1.58.3" "ed8ab4f2-384d-98da-81b7-d4466eaf0207" "k8s-combat-service:50051" "172.17.0.10:50051" outbound|50051||k8s-combat-service.default.svc.cluster.local 172.17.0.9:39800 10.101.204.13:50051 172.17.0.9:54012 - default
[2023-10-31T14:52:16.524Z] "POST /helloworld.Greeter/SayHello HTTP/2" 200 - via_upstream - "-" 12 61 28 26 "-" "grpc-go/1.58.3" "67a22028-dfb3-92ca-aa23-573660b30dd4" "k8s-combat-service:50051" "172.17.0.8:50051" outbound|50051||k8s-combat-service.default.svc.cluster.local 172.17.0.9:44580 10.101.204.13:50051 172.17.0.9:54012 - default
[2023-10-31T14:52:16.680Z] "POST /helloworld.Greeter/SayHello HTTP/2" 200 - via_upstream - "-" 12 61 2 2 "-" "grpc-go/1.58.3" "b4761d9f-7e4c-9f2c-b06f-64a028faa5bc" "k8s-combat-service:50051" "172.17.0.10:50051" outbound|50051||k8s-combat-service.default.svc.cluster.local 172.17.0.9:39800 10.101.204.13:50051 172.17.0.9:54012 - default
```

# 总结
本期的内容比较简单，主要和安装配置相关，下一期更新如何配置内部服务调用的超时、限流等功能。

其实目前大部分操作都是偏运维的，即便是后续的超时配置等功能都只是编写 yaml 资源。

但在生产使用时，我们会给开发者提供一个管理台的可视化页面，可供他们自己灵活配置这些原本需要在 `yaml` 中配置的功能。

![image.png](https://s2.loli.net/2023/10/31/B3TiC9rJwPbGVHQ.png)
其实各大云平台厂商都有提供类似的能力，比如阿里云的 EDAS 等。