/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bufio"
	"context"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	appsv1 "github.com/gsmini/gsmini-log-operator/api/v1"
	gsminiv1 "github.com/gsmini/gsmini-log-operator/api/v1"
	"io"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientsetCore "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GsminiLogReconciler reconciles a GsminiLog object
type GsminiLogReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ClientsetCore   *clientsetCore.Clientset
	sync.RWMutex    //并发写数据的时候可能会导致其他错误
	NotifyDeleteMap map[string]string
}

//+kubebuilder:rbac:groups=apps.gsmini.cn,resources=gsminilogs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.gsmini.cn,resources=gsminilogs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.gsmini.cn,resources=gsminilogs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GsminiLog object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *GsminiLogReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &gsminiv1.GsminiLog{}

	klog.Infof("[Reconcile call  start][ns:%v][GsminiLog:%v]", req.Namespace, req.Name)
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Errorf("[ Reconcile start missing be deleted][ns:%v][GsminiLog:%v]", req.Namespace, req.Name)
			// 如果错误是不存在，那么可能是到调谐这里 就被删了
			return reconcile.Result{}, nil
		}
		// 其它错误打印一下
		klog.Errorf("[ Reconcile start other error][err:%v][ns:%v][GsminiLog:%v]", err, req.Namespace, req.Name)
		return reconcile.Result{}, err
	}

	//真正获取pod的日志了
	//1-先获取当前namespace的所有pod
	// todo 这里是不合理的 需要把当前namespace所有的pod全部查询出来，目前只是查询limit=100 条
	//需要注意的是代码启动的时候已经存在的pod对他来讲也算新增的，并不是说要先启动代码再操作yaml文件触发新增
	watchHandler, err := r.ClientsetCore.CoreV1().Pods(instance.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{Limit: 100})

	if err != nil {
		return reconcile.Result{}, err
	}
	for {
		for event := range watchHandler.ResultChan() {
			//断言是来自pod事件的变化
			pod, ok := event.Object.(*apiv1.Pod)
			if !ok {
				continue
			}
			// 判断event的类型
			switch event.Type {

			//ADDED新增 如果是新增就开协程去收集每个pod下的container日志
			case watch.Added:
				klog.Errorf("[pod watch 事件类型：%v ]", watch.Added)
				go r.CollectPodLog(ctx, instance, instance.ObjectMeta.Namespace, pod.Name, pod.Spec.Containers)
			// DELETED删除
			//如果是删除 就往channel 放当前pod的name，消费者发现当前收到的channel中的名字是当前自己协助程序的名字就return 推出go程
			case watch.Deleted:
				//等待删除的podname
				r.Lock()
				r.NotifyDeleteMap[pod.Name] = pod.Name
				r.Unlock()

			default:
				klog.Errorf("[pod watch 事件类型：%v ]", event.Type)

			}

		}
	}

}

// SetupWithManager sets up the controller with the Manager.
func (r *GsminiLogReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.GsminiLog{}).
		Complete(r)
}

// CollectPodLog 针对单个pod中container的日志收集函数
func (r *GsminiLogReconciler) CollectPodLog(ctx context.Context, instance *gsminiv1.GsminiLog, PodNameSpace, PodName string, Containers []apiv1.Container) error {
	go func() {
		klog.Errorf("[pod 消费者开始消费:%s]", PodName)

		for _, value := range Containers {
			logOptions := &apiv1.PodLogOptions{
				Follow:    true, // 对应kubectl logs -f参数
				Container: value.Name,
			}
			err := func() error {
				//监控某个ContainerName日志
				stream, err := r.ClientsetCore.CoreV1().Pods(PodNameSpace).GetLogs(PodName, logOptions).Stream(context.TODO())
				if err != nil {

					return err
				}
				defer stream.Close()
				for {
					buffer := bufio.NewReader(stream)
					for {
						logString, err := buffer.ReadString('\n') // 读到一个换行就结束
						if err == io.EOF {                        // io.EOF表示文件的末尾
							break
						}

						//发送数据 这里不加锁的话并发写入导致的position数据不一致,会导致oss读取文件的position报错
						r.Lock()
						WriteOss(logString, instance)
						r.Unlock()
						FeiShu(logString, instance)
					}

				}
			}()
			if err != nil {
			}
		}

	}()
	//一秒检查一次
	tick := time.Tick(time.Second)
	select {
	case <-tick:
		_, ok := r.NotifyDeleteMap[PodName]
		//如果待删除map中有自己，说明当前go程可以退出了
		klog.Errorf("[pod 消费者退出:%s]", PodName)
		if ok {
			//等待10秒防止没完成数据写入的数据丢失
			time.Sleep(time.Second * 20)
			return nil
		}
	}
	return nil
}

func WriteOss(msg string, instance *gsminiv1.GsminiLog) {

	// 阿里云账号AccessKey拥有所有API的访问权限，风险很高。强烈建议您创建并使用RAM用户进行API访问或日常运维，请登录RAM控制台创建RAM用户。
	//client, err := oss.New("oss-cn-shenzhen.aliyuncs.com", "你的accesskey", "你的access secrect")
	// LogDestUri = "oss-cn-shenzhen.aliyuncs.com|accesskey|access secret|bucket name"
	alikeysL := strings.Split(instance.Spec.LogDestUri, "|")

	ossClient, err := oss.New(alikeysL[0], alikeysL[1], alikeysL[2])

	if err != nil {
		klog.Errorf("[ ossClient New error: %v]", err)
	}

	// 填写Bucket名称，例如examplebucket。
	bucket, err := ossClient.Bucket(alikeysL[3])
	if err != nil {
		klog.Errorf("[ ossClient Bucket init error: %v]", err)

	}
	// 填写不包含Bucket名称在内的Object的完整路径，例如2023/03/25.txt
	objectName := fmt.Sprintf("%s.txt", time.Now().Format("2006/01/02"))
	ok, err := bucket.IsObjectExist(objectName)
	if err != nil {
		klog.Errorf("[ oss IsObjectExist error: %v]", err)
	}
	//如果不存在直接第一次追加nextPos=0
	if !ok {
		_, err = bucket.AppendObject(objectName, strings.NewReader(msg), 0)
		if err != nil {
			klog.Errorf("[ oss AppendObject error: %v]", err)

		}
	} else {
		// 如果不是第一次追加上传，可以通过bucket.GetObjectDetailedMeta方法或上次追加返回值的X-Oss-Next-Append-Position的属性，获取追加位置。
		preopstions, err := bucket.GetObjectDetailedMeta(objectName)
		if err != nil {
			klog.Errorf("[ oss GetObjectDetailedMeta error: %v]", err)

		}
		nextPos, err := strconv.ParseInt(preopstions.Get("X-Oss-Next-Append-Position"), 10, 64)
		_, err = bucket.AppendObject(objectName, strings.NewReader(msg), nextPos)
		if err != nil {
			klog.Errorf("[ oss AppendObject error: %v]", err)

		}
	}

}

func FeiShu(msg string, instance *gsminiv1.GsminiLog) {
	//从kubectl apply -f 的yaml中取
	msg = strings.ReplaceAll(msg, "\\", "") //数据格式兼容处理
	msg = strings.ReplaceAll(msg, "\n", "")
	msg = strings.ReplaceAll(msg, "\r", "")
	msg = strings.ReplaceAll(msg, `"`, "")
	apiUrl := instance.Spec.LogReportUri
	contentType := "application/json"
	// data
	sendData := `{
		"msg_type": "text",
		"content": {"text": "` + "消息通知:" + msg + `"}
	}`
	// request
	result, err := http.Post(apiUrl, contentType, strings.NewReader(sendData))
	if err != nil {
		fmt.Printf("post failed, err:%v\n", err)
		return
	}
	//data, _ := io.ReadAll(result.Body)
	//if result.StatusCode != 200 {
	//	fmt.Println(string(data))
	//}

	defer result.Body.Close()
}
