package lease

import (
	"sort"
	"time"

	"github.com/mit-drl/goop"
	"github.com/mit-drl/goop/solvers"
)

type MIPSolver struct {
	method string
}

type Tuple3 struct {
	a int
	b int
	c int
}
type Tuple3List []Tuple3

func (s Tuple3List) Len() int      { return len(s) }
func (s Tuple3List) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s Tuple3List) Less(i, j int) bool {
	if s[i].a-s[i].b != s[j].a-s[j].b {
		return s[i].a-s[i].b < s[j].a-s[j].b
	}
	if s[i].b != s[j].b {
		return -s[i].b < -s[j].b
	}

	return -s[i].c < -s[j].c
}

func (ms *MIPSolver) BatchFastCheckIfPackable(requireResourceList []int,
	requireBlockList []int,
	maximumBlockList []int,
	existingSolution []int,
	resourceNumList []int) (bool, []int) {
	if len(maximumBlockList) == 0 {
		return true, existingSolution
	}
	maximumBlock := max(maximumBlockList)
	solution := []int{}
	for i := int(0); i < maximumBlock; i++ {
		if i < int(len(existingSolution)) {
			solution = append(solution, existingSolution[i])
		} else {
			solution = append(solution, resourceNumList[i])
		}
	}
	resourceTupleList := Tuple3List{}

	for i := int(0); i < int(len(maximumBlockList)); i++ {
		resourceTuple := Tuple3{maximumBlockList[i], requireBlockList[i], requireResourceList[i]}
		resourceTupleList = append(resourceTupleList, resourceTuple)
	}
	sort.Sort(resourceTupleList)
	feasible, cnt := false, 0
	for _, v := range resourceTupleList {
		feasible, cnt = false, 0
		maximumBlock := v.a
		requiredBlock := v.b
		requiredResource := v.c
		for i := maximumBlock - 1; i >= int(0); i-- {
			if solution[i] >= requiredResource {
				cnt++
				solution[i] -= requiredResource
			}
			if cnt == int(requiredBlock) {
				feasible = true
				break
			}
		}
		if feasible == false {
			break
		}
	}
	return feasible, solution
}

func (ms *MIPSolver) JobSelection(requireResourceList []int,
	requireBlockList []int,
	maximumBlockList []int,
	resourceNumList []int) (bool, [][]int) {
	maximumBlock := max(maximumBlockList)
	m := goop.NewModel()
	varLength := len(requireResourceList) * int(maximumBlock)
	X := []*goop.Var{}
	for i := 0; i < varLength; i++ {
		binVar := m.AddBinaryVar()
		X = append(X, binVar)
	}
	// job wise
	for i := 0; i < len(requireResourceList); i++ {
		tempVar := []*goop.Var{}
		for j := i * int(maximumBlock); j < i*int(maximumBlock)+int(maximumBlockList[i]); j++ {
			tempVar = append(tempVar, X[j])
		}
		if len(tempVar) > 0 {
			tempVarSum := goop.Sum(tempVar[0])
			for j := 1; j < len(tempVar); j++ {
				tempVarSum = goop.Sum(tempVarSum, tempVar[j])
			}
			m.AddConstr(tempVarSum.Eq(goop.K(requireBlockList[i])))
			if maximumBlockList[i] < maximumBlock {
				tempVar2 := []*goop.Var{}
				for j := i*int(maximumBlock) + int(maximumBlockList[i]); j < (i+1)*int(maximumBlock); j++ {
					tempVar2 = append(tempVar2, X[j])
				}
				if len(tempVar2) > 0 {
					tempVarSum2 := goop.Sum(tempVar2[0])
					for j := 1; j < len(tempVar2); j++ {
						tempVarSum2 = goop.Sum(tempVarSum2, tempVar2[j])
					}
					m.AddConstr(tempVarSum2.Eq(goop.K(0)))
				}

			}
		}
	}
	for i := 0; i < int(maximumBlock); i++ {
		tempVar := []*goop.Var{}
		cofList := []float64{}
		for j := i; j < varLength; j += int(maximumBlock) {
			tempVar = append(tempVar, X[j])
			cofList = append(cofList, float64(requireResourceList[j/int(maximumBlock)]))

		}

		if len(tempVar) > 0 {
			tempVarSum := goop.Sum(tempVar[0].Mult(cofList[0]))
			for j := 1; j < len(tempVar); j++ {
				tempVarSum = goop.Sum(tempVarSum, tempVar[j].Mult(cofList[j]))
			}
			m.AddConstr(goop.Sum(tempVarSum).LessEq(goop.K(resourceNumList[i])))
		}
	}
	obj := goop.Sum(X[0])
	for j := 0; j < varLength; j += int(maximumBlock) {
		obj = goop.Sum(X[j].Mult(float64(requireResourceList[j/int(maximumBlock)])))
	}
	m.SetObjective(obj, goop.SenseMinimize)
	m.SetTimeLimit(time.Duration(100))
	sol, err := m.Optimize(solvers.NewGurobiSolver())
	if err != nil {
		// panic("Should not have an error")
		return false, [][]int{}
	}
	solutionMatrix := make([][]int, len(requireResourceList))
	for i := 0; i < len(requireResourceList); i++ {
		start := int(i) * maximumBlock
		solution := make([]int, maximumBlockList[i])
		for j := start; j < start+maximumBlockList[i]; j++ {
			solution[j-start] = int(sol.Value(X[j]))
		}
		solutionMatrix[i] = solution
	}
	// for i := 0; i < len(X); i++ {
	// 	fmt.Println(i, sol.Value(X[i]))
	// }
	return true, solutionMatrix
}

func (ms *MIPSolver) FastCheckIfPackable(requireResource int,
	requireBlock int,
	maximumBlock int,
	existingSolution []int,
	resourceNumList []int) (bool, []int) {
	if requireBlock > maximumBlock {
		return false, existingSolution
	}
	feasibleBlock := int(0)
	if len(existingSolution) < int(maximumBlock) {
		for i := len(existingSolution); i < int(maximumBlock); i++ {
			existingSolution = append(existingSolution, resourceNumList[i])
		}
	}

	for i := 0; i < int(maximumBlock); i++ {
		if existingSolution[i] >= requireResource {
			feasibleBlock++
		}
	}
	feasible := feasibleBlock >= requireBlock
	if feasible {
		for i := int(maximumBlock) - 1; i >= 0; i-- {
			existingSolution[i] -= requireResource
			requireBlock--
		}
	}
	return feasible, existingSolution
}

//////////////////////// no preempt ////////////////////////

type Tuple4 struct {
	a int
	b int
	c int
	d int
}
type Tuple4List []Tuple4

func (s Tuple4List) Len() int      { return len(s) }
func (s Tuple4List) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s Tuple4List) Less(i, j int) bool {
	if s[i].a-s[i].b != s[j].a-s[j].b {
		return s[i].a-s[i].b < s[j].a-s[j].b
	}
	if s[i].b != s[j].b {
		return -s[i].b < -s[j].b
	}

	return -s[i].c < -s[j].c
}

type NoPreemptMIPSolver struct {
	method string
}

func (ms *NoPreemptMIPSolver) BatchFastCheckIfPackable(requireResourceList []int,
	requireBlockList []int,
	maximumBlockList []int,
	existingSolution []int,
	resourceNumList []int) (bool, []int) {
	if len(maximumBlockList) == 0 {
		return true, existingSolution
	}
	maximumBlock := max(maximumBlockList)
	solution := []int{}
	for i := int(0); i < maximumBlock; i++ {
		if i < int(len(existingSolution)) {
			solution = append(solution, existingSolution[i])
		} else {
			solution = append(solution, resourceNumList[i])
		}
	}
	resourceTupleList := Tuple3List{}

	for i := int(0); i < int(len(maximumBlockList)); i++ {
		resourceTuple := Tuple3{maximumBlockList[i], requireBlockList[i], requireResourceList[i]}
		resourceTupleList = append(resourceTupleList, resourceTuple)
	}
	sort.Sort(resourceTupleList)
	feasible, cnt := false, 0
	for _, v := range resourceTupleList {
		feasible, cnt = false, 0
		maximumBlock := v.a
		requiredBlock := v.b
		requiredResource := v.c
		// for i := maximumBlock - 1; i >= int(0); i-- {
		for i := 0; i < int(maximumBlock); i++ {
			if solution[i] >= requiredResource {
				cnt++
				solution[i] -= requiredResource
			}
			if cnt == int(requiredBlock) {
				feasible = true
				break
			}
		}
		if feasible == false {
			break
		}
	}
	return feasible, solution
}

func (ms *NoPreemptMIPSolver) BatchFastJobSelection(requireResourceList []int,
	requireBlockList []int,
	maximumBlockList []int,
	existingSolution []int,
	resourceNumList []int) (bool, [][]int) {
	if len(maximumBlockList) == 0 {
		return true, [][]int{}
	}
	maximumBlock := max(maximumBlockList)
	solution := []int{}
	for i := 0; i < int(maximumBlock); i++ {
		if i < len(existingSolution) {
			solution = append(solution, existingSolution[i])
		} else {
			solution = append(solution, resourceNumList[i])
		}
	}
	solutionMatrix := make([][]int, len(maximumBlockList))

	resourceTupleList := Tuple4List{}
	for i := 0; i < len(maximumBlockList); i++ {
		resourceTuple := Tuple4{maximumBlockList[i], requireBlockList[i], requireResourceList[i], int(i)}
		resourceTupleList = append(resourceTupleList, resourceTuple)
		// solutionMatrix = append(solutionMatrix, []int{})
	}
	sort.Sort(resourceTupleList)
	feasible, cnt := false, 0

	for _, v := range resourceTupleList {
		feasible, cnt = false, 0
		maximumBlock := v.a
		requiredBlock := v.b
		requiredResource := v.c
		idx := v.d
		cacheSolution := make([]int, maximumBlock)
		for i := 0; i < int(maximumBlock); i++ {
			if solution[i] >= requiredResource {
				cnt++
				solution[i] -= requiredResource
				cacheSolution[i] = int(1)
			}
			if cnt == int(requiredBlock) {
				feasible = true
				break
			}
		}
		if feasible == false {
			break
		}
		solutionMatrix[idx] = cacheSolution
	}
	return feasible, solutionMatrix
}
