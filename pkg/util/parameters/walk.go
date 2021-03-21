package parameters

// ReplaceFn defines the replace function
type ReplaceFn func(value string) (interface{}, error)

// Walk walks over an interface and replaces keys that match the match function with the replace function
func Walk(d map[interface{}]interface{}, replace ReplaceFn) error {
	return doWalk(d, replace)
}

// WalkStringMap walks over an interface and replaces keys that match the match function with the replace function
func WalkStringMap(d map[string]interface{}, replace ReplaceFn) error {
	return doWalk(d, replace)
}

func doWalk(d interface{}, replace ReplaceFn) error {
	var err error

	switch t := d.(type) {
	case []interface{}:
		for idx, val := range t {
			value, ok := val.(string)
			if ok == false {
				err = doWalk(val, replace)
				if err != nil {
					return err
				}

				continue
			}

			t[idx], err = replace(value)
			if err != nil {
				return err
			}
		}
	case map[string]interface{}:
		for key, v := range t {
			value, ok := v.(string)
			if ok == false {
				err = doWalk(v, replace)
				if err != nil {
					return err
				}

				continue
			}

			t[key], err = replace(value)
			if err != nil {
				return err
			}
		}
	case map[interface{}]interface{}:
		for k, v := range t {
			value, ok := v.(string)
			if ok == false {
				err = doWalk(v, replace)
				if err != nil {
					return err
				}

				continue
			}

			t[k], err = replace(value)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
