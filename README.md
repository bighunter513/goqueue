# goqueue
an Bounded Lock Free Queue use array as circle queue 

clone from https://github.com/yireyun/go-queue 
and changed little things, add some comments

## run test case
make test ## test all cases

make lfqueue ## only test lfqueue

## how to use 
q := NewQueue(1024)

ok, cnt := qq.Put(val) 

if ok {

}

val, ok, cnt := qq.Get()

if ok {

}

put & get both can be failed, so caller must check and do try thing



