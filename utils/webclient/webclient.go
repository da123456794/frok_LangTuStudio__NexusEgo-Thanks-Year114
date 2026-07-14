package webclient

import (
    "net/http"

    types "nexus/defines"
)

type Webclient struct {
    Client       *http.Client
    Client_id    string
    Replace_file bool
    Import_file  bool
    Name         string
}

func NewWebclient() *Webclient {
    return &Webclient{
        Client:       &http.Client{},
        Client_id:    "",
        Replace_file: false,
        Import_file:  false,
        Name:         "",
    }
}

func (client *Webclient) Encode(payload []byte) []byte {
    if payload == nil {
        return nil
    }
    out := make([]byte, len(payload))
    copy(out, payload)
    return out
}

func (client *Webclient) postTask(path string, payload any, out any) {}

func (client *Webclient) NewClient_id() string {
    return client.Client_id
}

func (client *Webclient) Login() {}

func (client *Webclient) Reconnect() {}

func (client *Webclient) Read_permission() {
    client.Replace_file = false
    client.Import_file = false
}

func (client *Webclient) Check_task(task_id string, user_id string) bool {
    return false
}

func (client *Webclient) Start_task_import_file(task_id string, user_id string) (bool, string) {
    return true, ""
}

func (client *Webclient) Finish_task_import_file(task_id string, user_id string) (bool, string) {
    return true, ""
}

func (client *Webclient) Get_new_task() (bool, types.Task, string) {
    return false, types.Task{}, ""
}

func (client *Webclient) Update_task_now_operation(task_id string, user_id string, now_operation string) bool {
    return true
}

func (client *Webclient) Update_task_operation(task_id string, user_id string, operation_app int, operation_max int, current_time float64, estimated_time float64) bool {
    return true
}

func (client *Webclient) Stop_task_import_file_by_no_op(task_id string, user_id string) (bool, string) {
    return true, ""
}