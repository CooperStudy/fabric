package a

import (
	"fmt"
	"github.com/Knetic/govaluate"
	"testing"
)

/*

支持任意C风格的字符串或表达式计算判断值。
为何无法直接在代码中写表达式？
有时，在运行前你不知道表达式会是什么样子，或者你想要表达式参数可配。或许你的应用中有跑着很多数据，
你希望用户在向数据库提交代码钱先做校验。亦或你的监控架构能收集很多Metric的信息，
再判断是否有报警时间发生，但每种报警监控场景又是不同的。
很多人都会基于自己的业务需求实现半成熟的表达式判断代码，
但是不够完善。或者在可执行文件中，将表达式写死，即使他们知道其实表达式是可变的。
这些方案或许可行，但是他们开发需要时间，用户也需要学习成本，并且在需求变化时可能会触发技术bug。
这个库可以cover所有C风格的表达式计算，所以你不必再重复造轮子了。
 */
func Test1(t *testing.T){
	expression, err := govaluate.NewEvaluableExpression( "10 < 0" )
	result, err := expression.Evaluate( nil )
	fmt.Println(result,err)
}
/*
表达式中加入参数
 */
func Test2(t *testing.T){
	expression, err := govaluate.NewEvaluableExpression( "foo > 0" )

	parameters := make ( map [ string ] interface {}, 8 )
	parameters[ "foo" ] = -1

	result, err := expression.Evaluate(parameters)
	fmt.Println("result",result)
	fmt.Println("err",err)
}
/*
那如何在表达式中引入复杂的数***算呢
 */
func Test3(t *testing.T) {
	expression, err := govaluate.NewEvaluableExpression( "(requests_made * requests_succeeded / 100) > 1" )
	parameters := make ( map [ string ] interface {}, 8 )
	parameters[ "requests_made" ] = 100
	parameters[ "requests_succeeded" ] = 2

	result, err := expression.Evaluate(parameters)

	fmt.Println("result",result)
	fmt.Println("err",err)
}

/*
你想进行一个冒烟测试，表达式该如何书写呢？
 */

func Test4(t *testing.T) {
	expression, err := govaluate.NewEvaluableExpression( "http_response_body == 'service is ok'" )
	parameters := make ( map [ string ] interface {}, 8 )
	parameters[ "http_response_body" ] = "service is ok"
	result, err := expression.Evaluate(parameters)
	fmt.Println("result",result,"err",err)
}
/*
实数值型计算也是支持的
 */

func Test5(t *testing.T) {
	expression, err := govaluate.NewEvaluableExpression( "(mem_used / total_mem) * 100" )
	parameters := make ( map [ string ] interface {}, 8 )
	parameters[ "total_mem" ] = 1024
	parameters[ "mem_used" ] = 51
	result, err := expression.Evaluate(parameters)
	fmt.Println("result",result,"err",err) //50
	fmt.Printf("%T\n",result)
	a := result.(float64)
	fmt.Println("a",a)
}
/*
函数可以接受任意数量的参数，正确地处理嵌套函数，并且参数可以是任何类型（即使此库的任何运算符都不支持对该类型的计算
例如，下例中每个函数的用法都是有效的（假定给定了适当的函数和参数）：
 */
func Test6(t *testing.T) {
	functions := map [ string ]govaluate.ExpressionFunction {
		"strlen" : func (args ... interface {}) ( interface {}, error) {
			length := len (args[0].(string))
			return ( float64 )(length), nil
		},
	}

	expString := "strlen('someReallyLongInputString')>16"
	expression, _ := govaluate.NewEvaluableExpressionWithFunctions(expString, functions)

	result, err := expression.Evaluate( nil )
	fmt.Println("result",result,"err",err)
}

func Test7(t *testing.T) {
	policy := []byte{18,8,18 ,6, 8 ,1 ,18, 2, 8, 0, 26, 19 ,18 ,17, 10 ,13 ,109, 97, 115, 116, 101, 114, 79 ,114, 103, 49, 77 ,83, 80 ,16 ,3}


	fmt.Println(string(policy))
}