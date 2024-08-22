package upload_utils

import (
	"container/heap"
)

// An IntHeap is a min-heap of ints.
type IntHeap []int

func NewHeap(s []int) *IntHeap {
	h := IntHeap(s)
	heap.Init(&h)
	return &h
}

func (h IntHeap) Len() int           { return len(h) }
func (h IntHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h IntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *IntHeap) Push(x any) {
	*h = append(*h, x.(int))
}

func (h *IntHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *IntHeap) Peek() int {
	if len(*h) == 0 {
		return 0
	}
	return (*h)[0]
}

func (h *IntHeap) Remove(x any) {
	toRemove := x.(int)
	for i := 0; i < len(*h); i++ {
		n := (*h)[i]
		if n == toRemove {
			(*h)[i], (*h)[len(*h)-1] = (*h)[len(*h)-1], (*h)[i]
			(*h) = (*h)[:len(*h)-1]
			i--
		}
	}
	heap.Init(h)
}

// FindMinMissingInteger finds the minimum missing integer in a sorted array
func FindMinMissingInteger(arr []int) int {
	left, right := 0, len(arr)

	for left < right {
		mid := (left + right) / 2

		// Check if arr[mid] is equal to the expected value mid + 1
		if arr[mid] == mid+1 {
			left = mid + 1
		} else {
			right = mid
		}
	}

	// The first missing number is left + 1
	return left + 1
}
