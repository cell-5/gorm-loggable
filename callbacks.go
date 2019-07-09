package loggable

import (
	"encoding/json"
	"reflect"

	"github.com/gofrs/uuid"
	"github.com/jinzhu/gorm"
)

var im = newIdentityManager()

const (
	actionCreate = "create"
	actionUpdate = "update"
	actionDelete = "delete"
)

type UpdateDiff map[string]interface{}

func (p *Plugin) trackEntity(scope *gorm.Scope) {
	if !isLoggable(scope.Value) || !isEnabled(scope.Value) {
		return
	}

	v := reflect.Indirect(reflect.ValueOf(scope.Value))

	pkName := scope.PrimaryField().Name
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			sv := reflect.Indirect(v.Index(i))
			el := sv.Interface()
			if !isLoggable(el) {
				continue
			}

			im.save(el, sv.FieldByName(pkName))
		}
		return
	}

	m := v.Interface()
	if !isLoggable(m) {
		return
	}

	im.save(scope.Value, scope.PrimaryKeyValue())
}

func (p *Plugin) addCreated(scope *gorm.Scope) {
	if isLoggable(scope.Value) && isEnabled(scope.Value) {
		addRecord(scope, actionCreate)
	}
}

func (p *Plugin) addUpdated(scope *gorm.Scope) {
	if !isLoggable(scope.Value) || !isEnabled(scope.Value) {
		return
	}

	if p.opts.lazyUpdate {
		record, err := p.GetLastRecord(interfaceToString(scope.PrimaryKeyValue()), false)
		if err == nil {
			if isEqual(record.RawObject, scope.Value, p.opts.lazyUpdateFields...) {
				return
			}
		}
	}

	addUpdateRecord(scope, p.opts)
}

func (p *Plugin) addDeleted(scope *gorm.Scope) {
	if isLoggable(scope.Value) && isEnabled(scope.Value) {
		addRecord(scope, actionDelete)
	}
}

func addUpdateRecord(scope *gorm.Scope, opts options) error {
	cl, err := newChangeLog(scope, actionUpdate)
	if err != nil {
		return err
	}

	cl.RawDiff = "null"

	if opts.computeDiff {
		diff := computeUpdateDiff(scope)

		if diff != nil {
			jd, err := json.Marshal(diff)
			if err != nil {
				return err
			}

			cl.RawDiff = string(jd)
		}
	}

	if err = scope.DB().Create(cl).Error; err != nil {
		return err
	}

	idx, err := newChangeLogIndex(cl.ID, scope)
	if err != nil {
		return err
	}

	return scope.DB().Create(idx).Error
}

func newChangeLog(scope *gorm.Scope, action string) (*ChangeLog, error) {
	rawObject, err := json.Marshal(scope.Value)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &ChangeLog{
		ID:         id,
		Action:     action,
		ObjectID:   interfaceToString(scope.PrimaryKeyValue()),
		ObjectType: scope.GetModelStruct().ModelType.Name(),
		RawObject:  string(rawObject),
		RawMeta:    string(fetchChangeLogMeta(scope)),
		RawDiff:    "null",
	}, nil
}

func newChangeLogIndex(changeLogID uuid.UUID, scope *gorm.Scope) (*ChangeLogIndex, error) {
	userID, err := pluckUserID(fetchChangeLogMeta(scope))
	if err != nil {
		return nil, err
	}

	return &ChangeLogIndex{
		ChangeLogID: changeLogID,
		UserID:      userID,
	}, nil
}

func addRecord(scope *gorm.Scope, action string) error {
	cl, err := newChangeLog(scope, action)
	if err != nil {
		return nil
	}

	if err = scope.DB().Create(cl).Error; err != nil {
		return err
	}

	idx, err := newChangeLogIndex(cl.ID, scope)
	if err != nil {
		return nil
	}

	return scope.DB().Create(idx).Error
}

func computeUpdateDiff(scope *gorm.Scope) UpdateDiff {
	old := im.get(scope.Value, scope.PrimaryKeyValue())
	if old == nil {
		return nil
	}

	ov := reflect.ValueOf(old)
	nv := reflect.Indirect(reflect.ValueOf(scope.Value))
	names := getLoggableFieldNames(old)

	diff := make(UpdateDiff)

	for _, name := range names {
		ofv := ov.FieldByName(name).Interface()
		nfv := nv.FieldByName(name).Interface()
		if ofv != nfv {
			diff[name] = nfv
		}
	}

	return diff
}
