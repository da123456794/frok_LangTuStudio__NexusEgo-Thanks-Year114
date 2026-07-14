package describe

import "strings"

// JavaBlockString represents a Java block in string format
type JavaBlockString struct {
	name  string
	snbt  string
	props Props
}

func NewJavaBlockString(name, snbt string) *JavaBlockString {
	propsForSearch, err := PropsForSearchFromStr(snbt)
	var props Props
	if err != nil || propsForSearch == nil {
		props = Props{}
	} else {
		// Convert PropsForSearch to Props
		props = make(Props, 0, len(*propsForSearch))
		for _, propSearch := range *propsForSearch {
			// Convert PropValForSearch to PropVal based on type
			var propVal PropVal
			if propSearch.Value.HasType(PropValTypeUint8) {
				propVal = PropValUint8(propSearch.Value.Uint8Val() != 0)
			} else if propSearch.Value.HasType(PropValTypeInt32) {
				propVal = PropValFromInt32(propSearch.Value.Int32Val())
			} else {
				propVal = PropValFromString(propSearch.Value.StringVal())
			}
			props = append(props, struct {
				Name  string
				Value PropVal
			}{
				Name:  propSearch.Name,
				Value: propVal,
			})
		}
	}
	return &JavaBlockString{
		name:  name,
		snbt:  snbt,
		props: props,
	}
}

func (j *JavaBlockString) Name() string {
	return j.name
}

func (j *JavaBlockString) SNBT() string {
	return j.snbt
}

func (j *JavaBlockString) Props() Props {
	return j.props
}

func (j *JavaBlockString) ToNBT() map[string]any {
	return j.props.ToNBT()
}

func (j *JavaBlockString) String() string {
	if j.snbt == "" || j.snbt == "{}" {
		return j.name
	}
	return strings.ReplaceAll(j.name+"["+j.snbt+"]", "\"", "")
}
