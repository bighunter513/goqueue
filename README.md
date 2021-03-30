# goqueue
an Bounded Lock Free Queue use array as circle queue 

clone from https://github.com/yireyun/go-queue 
and changed little things, add some comments

## run test case
make test

## how to use 
q := NewQueue(1024)
q.put(xxx)
q.get(yyy)

put & get both can be failed, so caller must check and do try thing



