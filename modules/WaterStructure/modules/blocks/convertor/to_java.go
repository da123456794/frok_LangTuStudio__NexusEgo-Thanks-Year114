package convertor

import (
	"sync"

	"github.com/Yeah114/blocks/describe"
)

// ToJavaConvertor converts Bedrock blocks to Java blocks
type ToJavaConvertor struct {
	airBlock     *describe.JavaBlockString
	unknownBlock *describe.JavaBlockString
	baseNames    map[string]*ToJavaBaseNames
}

type ToJavaBaseNames struct {
	AirBlock     *describe.JavaBlockString
	UnknownBlock *describe.JavaBlockString

	legacyValuesMapping []*describe.JavaBlockString
	statesWithBlock     []struct {
		states    *describe.PropsForSearch
		javaBlock *describe.JavaBlockString
	}
	statesQuickMatch map[string]*describe.JavaBlockString
	mu               sync.RWMutex
}

func NewToJavaConverter(airBlock, unknownBlock *describe.JavaBlockString) *ToJavaConvertor {
	if airBlock == nil {
		airBlock = describe.NewJavaBlockString("air", "")
	}
	if unknownBlock == nil {
		unknownBlock = describe.NewJavaBlockString("air", "")
	}
	return &ToJavaConvertor{
		airBlock:     airBlock,
		unknownBlock: unknownBlock,
		baseNames:    map[string]*ToJavaBaseNames{},
	}
}

func (c *ToJavaConvertor) ensureBaseNameGroup(name string) *ToJavaBaseNames {
	if to, found := c.baseNames[name]; found {
		return to
	}
	to := &ToJavaBaseNames{
		AirBlock:            c.airBlock,
		UnknownBlock:        c.unknownBlock,
		legacyValuesMapping: make([]*describe.JavaBlockString, 0),
		statesWithBlock: make([]struct {
			states    *describe.PropsForSearch
			javaBlock *describe.JavaBlockString
		}, 0),
		statesQuickMatch: make(map[string]*describe.JavaBlockString),
		mu:               sync.RWMutex{},
	}
	c.baseNames[name] = to
	return to
}

func (c *ToJavaConvertor) getBaseNameGroup(name string) (baseGroup *ToJavaBaseNames, found bool) {
	group, found := c.baseNames[name]
	return group, found
}

func (c *ToJavaConvertor) AddAnchorByLegacyValue(name describe.BaseWithNameSpace, legacyValue uint16, javaBlock *describe.JavaBlockString, overwrite bool) (exist bool, conflictErr error) {
	baseNameGroup := c.ensureBaseNameGroup(name.BaseName())
	return baseNameGroup.addAnchorByLegacyValue(legacyValue, javaBlock, overwrite)
}

func (c *ToJavaConvertor) PreciseMatchByLegacyValue(name describe.BaseWithNameSpace, legacyValue uint16) (javaBlock *describe.JavaBlockString, found bool) {
	baseGroup, found := c.getBaseNameGroup(name.BaseName())
	if !found {
		return c.airBlock, false
	}
	return baseGroup.preciseMatchByLegacyValue(legacyValue)
}

func (c *ToJavaConvertor) TryBestSearchByLegacyValue(name describe.BaseWithNameSpace, legacyValue uint16) (javaBlock *describe.JavaBlockString, found bool) {
	baseGroup, found := c.getBaseNameGroup(name.BaseName())
	if !found {
		return c.airBlock, false
	}
	return baseGroup.fuzzySearchByLegacyValue(legacyValue)
}

func (c *ToJavaConvertor) AddAnchorByState(name describe.BaseWithNameSpace, states *describe.PropsForSearch, javaBlock *describe.JavaBlockString, overwrite bool) (exist bool, conflictErr error) {
	baseNameGroup := c.ensureBaseNameGroup(name.BaseName())
	return baseNameGroup.addAnchorByState(states, javaBlock, overwrite)
}

func (c *ToJavaConvertor) PreciseMatchByState(name describe.BaseWithNameSpace, states *describe.PropsForSearch) (javaBlock *describe.JavaBlockString, found bool) {
	baseGroup, found := c.getBaseNameGroup(name.BaseName())
	if !found {
		return c.airBlock, false
	}
	return baseGroup.preciseMatchByState(states)
}

func (c *ToJavaConvertor) TryBestSearchByState(name describe.BaseWithNameSpace, states *describe.PropsForSearch) (javaBlock *describe.JavaBlockString, score describe.ComparedOutput, matchAny bool) {
	baseGroup, found := c.getBaseNameGroup(name.BaseName())
	if !found {
		return c.airBlock, describe.ComparedOutput{}, false
	}
	return baseGroup.fuzzySearchByState(states)
}

// Implementation for ToJavaBaseNames
func (baseNameGroup *ToJavaBaseNames) addAnchorByLegacyValue(legacyValue uint16, javaBlock *describe.JavaBlockString, overwrite bool) (exist bool, conflictErr error) {
	if int(legacyValue+1) <= len(baseNameGroup.legacyValuesMapping) {
		if baseNameGroup.legacyValuesMapping[legacyValue] == nil || overwrite {
			baseNameGroup.legacyValuesMapping[legacyValue] = javaBlock
			return false, nil
		} else if baseNameGroup.legacyValuesMapping[legacyValue] != javaBlock && !overwrite {
			return true, nil
		} else {
			return true, nil
		}
	}
	baseNameGroup.mu.Lock()
	defer baseNameGroup.mu.Unlock()
	for int(legacyValue+1) > len(baseNameGroup.legacyValuesMapping) {
		baseNameGroup.legacyValuesMapping = append(baseNameGroup.legacyValuesMapping, nil)
	}
	baseNameGroup.legacyValuesMapping[legacyValue] = javaBlock
	return false, nil
}

func (baseNameGroup *ToJavaBaseNames) preciseMatchByLegacyValue(legacyValue uint16) (javaBlock *describe.JavaBlockString, found bool) {
	if int(legacyValue+1) <= len(baseNameGroup.legacyValuesMapping) {
		if javaBlock = baseNameGroup.legacyValuesMapping[legacyValue]; javaBlock == nil {
			return baseNameGroup.AirBlock, false
		} else {
			return javaBlock, true
		}
	} else {
		return baseNameGroup.AirBlock, false
	}
}

func (baseNameGroup *ToJavaBaseNames) fuzzySearchByLegacyValue(legacyValue uint16) (javaBlock *describe.JavaBlockString, found bool) {
	if int(legacyValue+1) <= len(baseNameGroup.legacyValuesMapping) {
		if javaBlock = baseNameGroup.legacyValuesMapping[legacyValue]; javaBlock != nil {
			return javaBlock, true
		}
	}
	if int(legacyValue+1) <= len(baseNameGroup.statesWithBlock) {
		return baseNameGroup.statesWithBlock[legacyValue].javaBlock, true
	}
	if len(baseNameGroup.statesWithBlock) > 0 {
		return baseNameGroup.statesWithBlock[0].javaBlock, true
	}
	return baseNameGroup.AirBlock, false
}

func (baseNameGroup *ToJavaBaseNames) addAnchorByState(states *describe.PropsForSearch, javaBlock *describe.JavaBlockString, overwrite bool) (exist bool, conflictErr error) {
	quickMatchStr := "{}"
	if states != nil {
		quickMatchStr = states.InPreciseSNBT()
	}
	baseNameGroup.mu.RLock()
	if currentJavaBlock, found := baseNameGroup.statesQuickMatch[quickMatchStr]; found {
		if currentJavaBlock == javaBlock || currentJavaBlock.String() == javaBlock.String() {
			baseNameGroup.mu.RUnlock()
			return true, nil
		} else if !overwrite {
			baseNameGroup.mu.RUnlock()
			return true, nil
		}
	}
	baseNameGroup.mu.RUnlock()
	baseNameGroup.mu.Lock()
	defer baseNameGroup.mu.Unlock()
	baseNameGroup.statesWithBlock = append(baseNameGroup.statesWithBlock, struct {
		states    *describe.PropsForSearch
		javaBlock *describe.JavaBlockString
	}{states: states, javaBlock: javaBlock})
	baseNameGroup.statesQuickMatch[quickMatchStr] = javaBlock
	return false, nil
}

func (baseNameGroup *ToJavaBaseNames) preciseMatchByState(states *describe.PropsForSearch) (javaBlock *describe.JavaBlockString, found bool) {
	quickMatchStr := states.InPreciseSNBT()
	baseNameGroup.mu.RLock()
	defer baseNameGroup.mu.RUnlock()
	javaBlock, found = baseNameGroup.statesQuickMatch[quickMatchStr]
	return javaBlock, found
}

func (baseNameGroup *ToJavaBaseNames) fuzzySearchByState(states *describe.PropsForSearch) (javaBlock *describe.JavaBlockString, score describe.ComparedOutput, matchAny bool) {
	quickMatchStr := states.InPreciseSNBT()
	baseNameGroup.mu.RLock()
	defer baseNameGroup.mu.RUnlock()
	javaBlock, found := baseNameGroup.statesQuickMatch[quickMatchStr]
	if found {
		sameCount := uint8(0)
		if states != nil {
			sameCount = uint8(states.NumProps())
		}
		return javaBlock, describe.ComparedOutput{Same: sameCount}, true
	}
	javaBlock = baseNameGroup.AirBlock
	matchAny = false
	for _, anchor := range baseNameGroup.statesWithBlock {
		newScore := anchor.states.Compare(states)
		if (!matchAny) || newScore.Same > score.Same || (newScore.Same == score.Same && ((newScore.Different + newScore.Redundant + newScore.Missing) < (score.Different + score.Redundant + score.Missing))) {
			score = newScore
			javaBlock = anchor.javaBlock
		}
		matchAny = true
	}
	return javaBlock, score, matchAny
}
