package pacing

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/brg-liuwei/gotools"
)

type PacingController struct {
	sync.Mutex
	m          *gotools.ExpiredMap
	pacing     int
	nInstances int
}

func newPacingController(pacing, nInstances int) *PacingController {
	return &PacingController{
		m:          gotools.NewExpiredMap(65536),
		pacing:     pacing,
		nInstances: nInstances,
	}
}

func NewPacingController(autoScalingGroupName, awsRegion string) *PacingController {
	offerGlobalPacing := 700 // 700 clicks per minite
	sess, err := session.NewSession(aws.NewConfig().WithRegion(awsRegion))
	if err != nil {
		fmt.Println("[AWS] AWS NewSession error: ", err)
		return newPacingController(offerGlobalPacing, 20) // by default, nInstances = 20
	}

	svc := autoscaling.New(sess)
	input := &autoscaling.DescribeAutoScalingGroupsInput{}
	input.SetAutoScalingGroupNames([]*string{
		&autoScalingGroupName,
	})

	output, err := svc.DescribeAutoScalingGroups(input)
	if err != nil {
		fmt.Println("[AWS] Invoke AWS API DescribeAutoScalingGroups error: ", err)
		return newPacingController(offerGlobalPacing, 20)
	}

	groups := output.AutoScalingGroups
	if len(groups) != 1 {
		fmt.Println("[AWS] Un-expected size of ", autoScalingGroupName, " ASG: ", len(groups))
		return newPacingController(offerGlobalPacing, 20)
	}

	nInstances := len(groups[0].Instances)
	if nInstances == 0 {
		fmt.Println("[AWS] Un-expected result of ", autoScalingGroupName, " ASG, instances size zero")
		return newPacingController(offerGlobalPacing, 20)
	}
	return newPacingController(offerGlobalPacing/nInstances, nInstances)
}

func (ctrl *PacingController) Add(uniqId string, now time.Time, count int) (newCount int) {
	key := fmt.Sprintf("%s_%d", uniqId, now.Minute())
	ctrl.Lock()
	defer ctrl.Unlock()

	iCount := ctrl.m.Get(key)
	iValue := reflect.ValueOf(iCount)
	if !iValue.IsValid() {
		ctrl.m.Put(key, count, 2*time.Minute)
		return count
	}

	count += int(iValue.Int())
	ctrl.m.Put(key, count, 2*time.Minute)
	return count
}

func (ctrl *PacingController) OverCap(uniqId, country string, manualPacing int, now time.Time) bool {
	key := fmt.Sprintf("%s_%d", uniqId, now.Minute())
	ctrl.Lock()
	iCount := ctrl.m.Get(key)
	ctrl.Unlock()

	iValue := reflect.ValueOf(iCount)
	if !iValue.IsValid() {
		return false
	}

	threshold := ctrl.pacing
	if country == "CN" {
		// 国内速度翻倍
		threshold *= 2
	}

	if manualPacing != -1 {
		// -1 means use default config
		threshold = manualPacing / ctrl.nInstances
	}

	return int(iValue.Int()) > threshold
}

func (ctrl *PacingController) Size() int {
	ctrl.Lock()
	defer ctrl.Unlock()
	return ctrl.m.Len()
}
