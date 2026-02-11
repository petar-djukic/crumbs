package types

import (
	"testing"
)

func TestDefaultValue(t *testing.T) {
	tests := []struct {
		valueType string
		wantVal   any
		wantErr   error
	}{
		{ValueTypeCategorical, nil, nil},
		{ValueTypeText, "", nil},
		{ValueTypeInteger, int64(0), nil},
		{ValueTypeBoolean, false, nil},
		{ValueTypeTimestamp, nil, nil},
		{ValueTypeList, []string{}, nil},
		{"unknown", nil, ErrInvalidValueType},
	}
	for _, tt := range tests {
		t.Run(tt.valueType, func(t *testing.T) {
			val, err := DefaultValue(tt.valueType)
			if err != tt.wantErr {
				t.Errorf("DefaultValue(%q) error = %v, want %v", tt.valueType, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			// Handle []string comparison separately.
			if tt.valueType == ValueTypeList {
				slice, ok := val.([]string)
				if !ok {
					t.Errorf("DefaultValue(%q) = %T, want []string", tt.valueType, val)
				} else if len(slice) != 0 {
					t.Errorf("DefaultValue(%q) = %v, want empty slice", tt.valueType, val)
				}
				return
			}
			if val != tt.wantVal {
				t.Errorf("DefaultValue(%q) = %v, want %v", tt.valueType, val, tt.wantVal)
			}
		})
	}
}

func TestIsValidValueType(t *testing.T) {
	valid := []string{
		ValueTypeCategorical, ValueTypeText, ValueTypeInteger,
		ValueTypeBoolean, ValueTypeTimestamp, ValueTypeList,
	}
	for _, vt := range valid {
		if !IsValidValueType(vt) {
			t.Errorf("IsValidValueType(%q) = false, want true", vt)
		}
	}
	invalid := []string{"", "unknown", "float", "date"}
	for _, vt := range invalid {
		if IsValidValueType(vt) {
			t.Errorf("IsValidValueType(%q) = true, want false", vt)
		}
	}
}

func TestPropertyDefineCategory(t *testing.T) {
	tests := []struct {
		name      string
		valueType string
		catName   string
		wantErr   error
	}{
		{"valid categorical", ValueTypeCategorical, "high", nil},
		{"non-categorical type", ValueTypeText, "high", ErrInvalidValueType},
		{"empty name", ValueTypeCategorical, "", ErrInvalidName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Property{PropertyID: "prop-1", ValueType: tt.valueType}

			cat, err := p.DefineCategory(nil, tt.catName, 0)

			if err != tt.wantErr {
				t.Errorf("DefineCategory(%q) error = %v, want %v", tt.catName, err, tt.wantErr)
			}
			if err == nil {
				if cat == nil {
					t.Fatal("DefineCategory returned nil category on success")
				}
				if cat.PropertyID != p.PropertyID {
					t.Errorf("Category.PropertyID = %q, want %q", cat.PropertyID, p.PropertyID)
				}
				if cat.Name != tt.catName {
					t.Errorf("Category.Name = %q, want %q", cat.Name, tt.catName)
				}
			}
		})
	}
}

func TestPropertyGetCategoriesNonCategorical(t *testing.T) {
	p := &Property{ValueType: ValueTypeText}
	_, err := p.GetCategories(nil)
	if err != ErrInvalidValueType {
		t.Errorf("GetCategories on text property: error = %v, want %v", err, ErrInvalidValueType)
	}
}
