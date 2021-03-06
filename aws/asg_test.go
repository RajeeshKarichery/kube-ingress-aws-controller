package aws

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

type asgtags map[string]string

func mockAutoScalingGroupDetails(name string, tags map[string]string) *autoScalingGroupDetails {
	return &autoScalingGroupDetails{
		name:         name,
		targetGroups: make([]string, 0),
		tags:         tags,
	}
}

func TestGetAutoScalingGroupByName(t *testing.T) {
	for _, test := range []struct {
		name      string
		givenName string
		responses autoscalingMockOutputs
		want      *autoScalingGroupDetails
		wantError bool
	}{
		{
			"success-call-single-asg",
			"foo",
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"foo": {"bar": "baz"}}), nil),
			},
			mockAutoScalingGroupDetails("foo", map[string]string{"bar": "baz"}),
			false,
		},
		{
			"success-call-multiple-asg",
			"d",
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			mockAutoScalingGroupDetails("d", map[string]string{"e": "f"}),
			false,
		},
		{
			"fail-to-match-single-asg",
			"miss",
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"foo": {"bar": "baz"}}), nil),
			},
			nil,
			true,
		},
		{
			"fail-to-match-multiple-asg",
			"miss",
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			nil,
			true,
		},
		{
			"autoscaling-api-failure",
			"dontcare",
			autoscalingMockOutputs{describeAutoScalingGroups: R(nil, dummyErr)},
			nil,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockAutoScalingClient{outputs: test.responses}
			got, err := getAutoScalingGroupByName(mockSvc, test.givenName)

			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				if !reflect.DeepEqual(test.want, got) {
					t.Errorf("unexpected result. wanted %+v, got %+v", test.want, got)
				}
			}
		})
	}
}

func TestGetAutoScalingGroupsByName(t *testing.T) {
	for _, test := range []struct {
		name       string
		givenNames []string
		responses  autoscalingMockOutputs
		want       map[string]*autoScalingGroupDetails
		wantError  bool
	}{
		{
			"success-call-single-asg",
			[]string{"foo"},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"foo": {"bar": "baz"}}), nil),
			},
			map[string]*autoScalingGroupDetails{
				"foo": mockAutoScalingGroupDetails("foo", map[string]string{"bar": "baz"}),
			},
			false,
		},
		{
			"success-call-multiple-asg",
			[]string{"a", "d"},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			map[string]*autoScalingGroupDetails{
				"a": mockAutoScalingGroupDetails("a", map[string]string{"b": "c"}),
				"d": mockAutoScalingGroupDetails("d", map[string]string{"e": "f"}),
			},
			false,
		},
		{
			"fail-to-match-single-asg",
			[]string{"miss"},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{"foo": {"bar": "baz"}}), nil),
			},
			nil,
			true,
		},
		{
			"fail-to-match-multiple-asg",
			[]string{"miss", "miss2"},
			autoscalingMockOutputs{
				describeAutoScalingGroups: R(mockDASGOutput(map[string]asgtags{
					"a": {"b": "c"},
					"d": {"e": "f"},
				}), nil),
			},
			nil,
			true,
		},
		{
			"autoscaling-api-failure",
			[]string{"dontcare"},
			autoscalingMockOutputs{describeAutoScalingGroups: R(nil, dummyErr)},
			nil,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockAutoScalingClient{outputs: test.responses}
			got, err := getAutoScalingGroupsByName(mockSvc, test.givenNames)

			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
				if !reflect.DeepEqual(test.want, got) {
					t.Errorf("unexpected result. wanted %v, got %v", test.want, got)
				}
			}
		})
	}
}

func TestAttach(t *testing.T) {
	for _, test := range []struct {
		name          string
		responses     autoscalingMockOutputs
		elbv2Response elbv2MockOutputs
		ownerTags     map[string]string
		wantError     bool
	}{
		{
			name: "describe-failed",
			responses: autoscalingMockOutputs{
				describeLoadBalancerTargetGroups: R(nil, dummyErr),
			},
			wantError: true,
		},
		{
			name: "describe-tags-failed",
			responses: autoscalingMockOutputs{
				attachLoadBalancerTargetGroups: R(nil, nil),
				describeLoadBalancerTargetGroups: R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil),
				detachLoadBalancerTargetGroups: R(nil, nil),
			},
			elbv2Response: elbv2MockOutputs{
				describeTags: R(nil, dummyErr),
			},
			wantError: true,
		},
		{
			name: "success-attach",
			responses: autoscalingMockOutputs{
				attachLoadBalancerTargetGroups: R(nil, nil),
				describeLoadBalancerTargetGroups: R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil)},
			elbv2Response: elbv2MockOutputs{
				describeTags: R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []*elbv2.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []*elbv2.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: false,
		},
		{
			name: "failed-attach",
			responses: autoscalingMockOutputs{
				attachLoadBalancerTargetGroups: R(nil, dummyErr),
				describeLoadBalancerTargetGroups: R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
					},
				}, nil),
			},
			elbv2Response: elbv2MockOutputs{
				describeTags: R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []*elbv2.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []*elbv2.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: true,
		},
		{
			name: "detach-obsolete",
			responses: autoscalingMockOutputs{
				attachLoadBalancerTargetGroups: R(nil, nil),
				describeLoadBalancerTargetGroups: R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("bar"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("baz"),
						},
					},
				}, nil),
				detachLoadBalancerTargetGroups: R(nil, nil),
			},
			elbv2Response: elbv2MockOutputs{
				describeTags: R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []*elbv2.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []*elbv2.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
						{
							ResourceArn: aws.String("bar"),
							Tags: []*elbv2.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
						{
							ResourceArn: aws.String("baz"),
							Tags:        []*elbv2.Tag{},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: false,
		},
		{
			name: "failed-detach",
			responses: autoscalingMockOutputs{
				attachLoadBalancerTargetGroups: R(nil, nil),
				describeLoadBalancerTargetGroups: R(&autoscaling.DescribeLoadBalancerTargetGroupsOutput{
					LoadBalancerTargetGroups: []*autoscaling.LoadBalancerTargetGroupState{
						{
							LoadBalancerTargetGroupARN: aws.String("foo"),
						},
						{
							LoadBalancerTargetGroupARN: aws.String("bar"),
						},
					},
				}, nil),
				detachLoadBalancerTargetGroups: R(nil, dummyErr),
			},
			elbv2Response: elbv2MockOutputs{
				describeTags: R(&elbv2.DescribeTagsOutput{
					TagDescriptions: []*elbv2.TagDescription{
						{
							ResourceArn: aws.String("foo"),
							Tags: []*elbv2.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
						{
							ResourceArn: aws.String("bar"),
							Tags: []*elbv2.Tag{
								{
									Key:   aws.String("owner"),
									Value: aws.String("true"),
								},
							},
						},
					},
				}, nil),
			},
			ownerTags: map[string]string{"owner": "true"},
			wantError: true,
		},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockAutoScalingClient{outputs: test.responses}
			mockElbv2Svc := &mockElbv2Client{outputs: test.elbv2Response}
			err := updateTargetGroupsForAutoScalingGroup(mockSvc, mockElbv2Svc, []string{"foo"}, "bar", test.ownerTags)
			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
			}
		})
	}

}

func TestDetach(t *testing.T) {
	for _, test := range []struct {
		name      string
		responses autoscalingMockOutputs
		wantError bool
	}{
		{"success-detach", autoscalingMockOutputs{detachLoadBalancerTargetGroups: R(nil, nil)},
			false},
		{"failed-detach", autoscalingMockOutputs{detachLoadBalancerTargetGroups: R(nil, dummyErr)},
			true},
	} {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			mockSvc := &mockAutoScalingClient{outputs: test.responses}
			err := detachTargetGroupsFromAutoScalingGroup(mockSvc, []string{"foo"}, "bar")
			if test.wantError {
				if err == nil {
					t.Error("wanted an error but call seemed to have succeeded")
				}
			} else {
				if err != nil {
					t.Fatal("unexpected error", err)
				}
			}
		})
	}

}
