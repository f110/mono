package web

import (
	"html/template"
	"time"
)

var (
	IndexTemplate      *template.Template
	DetailTemplate     *template.Template
	ServerInfoTemplate *template.Template
)

func init() {
	funcs := map[string]interface{}{
		"Duration": func(start *time.Time, end *time.Time) string {
			if end == nil || start == nil {
				return ""
			}
			return end.Sub(*start).String()
		},
	}

	IndexTemplate = template.Must(template.New("").Funcs(funcs).Parse(indexTemplate))
	DetailTemplate = template.Must(template.New("").Funcs(funcs).Parse(detailTemplate))
	ServerInfoTemplate = template.Must(template.New("").Funcs(funcs).Parse(infoTemplate))
}

const indexTemplate = `<html>
<head>
  <title>Build dashboard</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/semantic-ui@2.4.2/dist/semantic.min.css">
  <script src="https://code.jquery.com/jquery-3.4.1.min.js" integrity="sha256-CSXorXvZcTkaix6Yvo6HppcZGetbYMGWSFlBw8HfCJo=" crossorigin="anonymous"></script>
  <script src="https://cdn.jsdelivr.net/npm/semantic-ui@2.4.2/dist/semantic.min.js"></script>
  <style>
    i.amber.icon {color: #FFA000;}
  </style>
</head>
<body>

<div class="ui menu inverted huge">
  <div class="header item">
    <a href="/">Dashboard</a>
  </div>
  <div class="ui item dropdown simple">
    Repositories<i class="dropdown icon"></i>
    <div class="menu">
      {{- range .Repositories }}
      <a class="item">{{ .Name }}</a>
      {{- end }}
      <div class="ui divider"></div>
      <a class="item" onclick="newRepository();">New...</a>
      <div class="ui item dropdown simple">
        <i class="tiny trash icon"></i>Delete
        <div class="menu">
        {{- range .Repositories }}
        <a class="item" onclick="deleteRepository({{ .Id }}, '{{ .Name }}');">{{ .Name }}</a>
        {{- end }}
        </div>
      </div>
    </div>
  </div>

  <div class="ui item dropdown simple">
    Trusted User<i class="dropdown icon"></i>
    <div class="menu">
      {{- range .TrustedUsers }}
      <a class="item">{{ .Username }}</a>
      {{- end }}
      <div class="ui divider"></div>
      <a class="item" onclick="$('.ui.addUser.modal').modal({centered:false}).modal('show');">Add...</a>
    </div>
  </div>

  <div class="ui item dropdown simple">
    Run task<i class="dropdown icon"></i>
    <div class="menu">
      {{- range .Repositories }}
      <a class="item" onclick="startRunTask({{ .Id }}, '{{ .Name }}')">{{ .Name }}</a>
      {{- end }}
    </div>
  </div>

  <div class="item right">
    <a href="/server_info"><i class="info icon"></i>Info</a>
  </div>
</div>

<!-- modal -->
<div class="ui newRepo modal">
  <i class="close icon"></i>
  <div class="header">
    New Repository
  </div>
  <div class="content">
    <form class="ui form newRepo">
      <div class="field">
        <label>Name</label>
        <input type="text" name="name" placeholder="The name of the repository">
      </div>
      <div class="field">
        <label>URL</label>
        <input type="text" name="url" placeholder="URL of the repository (e.g https://github.com/f110/sandbox)">
      </div>
      <div class="field">
        <label>Clone URL</label>
        <input type="text" name="clone_url" placeholder="URL for cloning of the repository (e.g https://github.com/f110/sandbox.git)">
      </div>
      <div class="field">
        <div class="ui checkbox">
          <input type="checkbox" tabindex="0" class="hidden" name="private">
          <label>Private Repository</label>
        </div>
      </div>
      <button class="ui button" onclick="createRepository()">Add</button>
    </form>
  </div>
</div>

<div class="ui basic modal">
  <div class="ui icon header">
    <i class="archive icon"></i>
    Delete "<span class="repoName"></span>" repository
  </div>
  <div class="actions">
    <div class="ui red basic cancel inverted button">
      <i class="remove icon"></i>
      No
    </div>
    <div class="ui green ok inverted button">
      <i class="checkmark icon"></i>
      Yes
    </div>
  </div>
</div>

<div class="ui addUser modal">
  <i class="close icon"></i>
  <div class="header">
    Add Trusted User
  </div>
  <div class="content">
    <form class="ui form addUser" name="addUser">
      <div class="field">
        <label>GitHub Username</label>
        <input type="text" name="username" placeholder="octocat">
      </div>
      <button class="ui button" type="button" onclick="addTrustedUser()">Add</button>
    </form>
  </div>
</div>

<div class="ui runTask modal">
  <i class="close icon"></i>
  <div class="header">
    Run task
  </div>
  <div class="content">
    <form class="ui form runTask" name="runTask">
      <input type="hidden" name="repository_id">
      <div class="field">
        <label>Repository</label>
        <span class="repoName"></span>
      </div>
      <div class="field">
        <label>Task name</label>
        <input type="text" name="job_name">
      </div>
      <button class="ui button positive" type="button" onclick="runJob()">Run</button>
    </form>
  </div>
</div>
<!-- end of modal -->

<div class="ui container">
  {{- range .RepoAndTasks }}
  <h2 class="ui block header">
    <div class="ui grid">
      <div class="two column row">
        <div class="left floated column">{{ .Repo.Name }}</div>
      </div>
    </div>
  </h2>

  <div class="ui container">
    <table class="ui selectable striped table">
      <thead>
        <tr>
          <th>#</th>
          <th>Job</th>
          <th>OK</th>
          <th>Log</th>
          <th>Rev</th>
          <th>Trigger</th>
          <th>Start at</th>
          <th>Duration</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
		{{- range .Tasks }}
        <tr>
          <td><a href="/task/{{ .Id }}">{{ .Id }}</a></td>
          <td class="buildinfo" data-content="Bazel version: {{ .BazelVersion }}">{{ .JobName }}</td>
          <td>{{ if .FinishedAt }}{{ if .Success }}<i class="green check icon"></i>{{ else }}<i class="red attention icon"></i>{{ end }}{{ else }}<i class="sync amber alternate icon"></i>{{ end }}</td>
          <td>{{ if .LogFile }}<a href="/logs/{{ .LogFile }}">text</a>{{ end }}</td>
          <td><a href="{{ .RevisionUrl }}">{{ if .Revision }}{{ slice .Revision 0 6 }}{{ end }}</a></td>
          <td>{{ .Via }}</td>
          <td>{{ if .StartAt }}{{ .StartAt.Format "2006/01/02 15:04:06" }}{{ end }}</td>
          <td>{{ Duration .StartAt .FinishedAt }}</td>
          <td>{{ if .FinishedAt }}<a href="#" onclick="redoTask({{ .Id }})"><i class="amber redo icon"></i></a>{{ end }}</td>
        </tr>
        {{- end }}
      </tbody>
    </table>
    {{- end }}
  </div>
</div>

<script>
const apiHost = {{ .APIHost }};

$('.ui.checkbox')
  .checkbox()
;

$('.buildinfo')
  .popup({})
;

function newRepository() {
	$('.ui.newRepo.modal').modal({centered:false}).modal('show');
}

function createRepository() {
	var f = document.querySelector('.ui.form.newRepo');
	var params = new URLSearchParams();
	params.append("name", f.name.value);
	params.append("url", f.url.value);
	params.append("clone_url", f.clone_url.value);
    params.append("private", f.private.value);
	fetch('/new_repo', {
		method: 'POST',
		body: params,
	});
}

function addTrustedUser() {
	var f = document.querySelector('.ui.form.addUser');
	var params = new URLSearchParams();
	params.append("username", f.username.value);
	fetch('/add_trusted_user', {
		method: 'POST',
		body: params,
	}).then(response => {
		if (response.ok) {
			window.location.reload(false);
		}
	});
}

function deleteRepository(id, name) {
  var e = document.querySelector('span.repoName');
  e.textContent = name;
  $('.ui.basic.modal').modal({
    onApprove: function() {
		var params = new URLSearchParams();
		params.append("id", id);
		fetch('/delete_repo', {
			method: 'POST',
			body: params,
		});
	},
  }).modal('show');
}

function redoTask(id) {
  var params = new URLSearchParams();
  params.append("task_id", id);
  fetch(apiHost+"/redo",{
    mode: 'cors',
    method: 'POST',
    credentials: 'include',
    body: params,
  }).then(response => {
    if (response.ok) {
      window.location.reload(false);
    }
  });
}

function startRunTask(id, name) {
  var elms = document.querySelectorAll('span.repoName');
  elms.forEach(function (e) {
    e.textContent = name;
  });
  var e = document.querySelector('input[name=repository_id]');
  e.value = id;
  $('.ui.runTask.modal').modal({centered:false}).modal('show');
}

function runJob() {
  var f = document.querySelector('.ui.form.runTask');
  var params = new URLSearchParams();
  params.append("repository_id", f.repository_id.value);
  params.append("job_name", f.job_name.value);
  fetch(apiHost+'/run', {
    mode: 'cors',
    method: 'POST',
    credentials: 'include',
    body: params,
  }).then(response => {
    if (response.ok) {
      window.location.reload(false);
    }
  });
}

</script>
</body>
</html>`

const detailTemplate = `<html>
<head>
  <title>Build dashboard</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/semantic-ui@2.4.2/dist/semantic.min.css">
  <script src="https://code.jquery.com/jquery-3.4.1.min.js" integrity="sha256-CSXorXvZcTkaix6Yvo6HppcZGetbYMGWSFlBw8HfCJo=" crossorigin="anonymous"></script>
  <script src="https://cdn.jsdelivr.net/npm/semantic-ui@2.4.2/dist/semantic.min.js"></script>
  <style>
    i.amber.icon {color: #FFA000;}
    .ui.amber.table {border-top: 0.2em solid #FFA000;}
    .ui.amber.button {background-color: #FFA000; color: #FFFFFF;}
  </style>
</head>
<body>

<div class="ui menu inverted huge">
  <div class="header item">
    <a href="/">Dashboard</a>
  </div>
  <div class="ui item simple">
    {{ .Task.Repository.Name }}
  </div>

  <div class="item right">
    <a href="/server_ionfo"><i class="info icon"></i>Info</a>
  </div>
</div>

<div class="ui container">
  <table class="ui {{ if .Task.FinishedAt }}{{ if .Task.Success }}green{{ else }}red{{ end }}{{ else }}amber{{ end }} table definition">
    <tbody>
    <tr>
      <td>id</td>
      <td>{{ .Task.Id }}</td>
    </tr>
    <tr>
      <td>Status</td>
      <td>{{ if .Task.FinishedAt }}{{ if .Task.Success }}<i class="green check icon"></i>Success{{ else }}<i class="red attention icon"></i>Failure{{ end }}{{ else }}<i class="amber sync alternate icon"></i>Running{{ end }}</td>
    </tr>
    <tr>
      <td>Job</td>
      <td>{{ .Task.JobName }}</td>
    </tr>
    <tr>
      <td>Revision</td>
      <td><a href="{{ .Task.RevisionUrl }}">{{ if .Task.Revision }}{{ slice .Task.Revision 0 6 }}{{ end }}</a></td>
    </tr>
    <tr>
      <td>Started at</td>
      <td>{{ if .Task.StartAt }}{{ .Task.StartAt.Format "2006/01/02 15:04:06" }}{{ end }}</td>
    </tr>
    <tr>
      <td>Duration</td>
      <td>{{ Duration .Task.StartAt .Task.FinishedAt }}</td>
    </tr>
    <tr>
      <td>Node</td>
      <td>{{ .Task.Node }}</td>
    </tr>
    <tr>
      <td>Trigger</td>
      <td>{{ .Task.Via }}</td>
    </tr>
    <tr>
      <td>Bazel version</td>
      <td>{{ .Task.BazelVersion }}</td>
    </tr>
    <tr>
      <td>Container</td>
      <td>{{ .Task.Container }}</td>
    </tr>
    <tr>
      <td>CPU / Memory Limit</td>
      <td>{{ .Job.CPULimit }} / {{ .Job.MemoryLimit }}</td>
    </tr>
    <tr>
      <td>Log</td>
      <td>{{ if .Task.LogFile }}<a href="/logs/{{ .Task.LogFile }}">text</a>{{ end }}</td>
    </tr>
    <tr>
      <td>Job manifest</td>
      <td><a href="/manifest/{{ .Task.Id }}">yaml</a></td>
    </tr>
    </tbody>
  </table>

  {{ if .Task.FinishedAt }}<a class="ui button amber" href="#" onclick="redoTask({{ .Task.Id }})">Restart</a>{{ end }}
</div>

<script>
const apiHost = {{ .APIHost }};

function redoTask(id) {
  var params = new URLSearchParams();
  params.append("task_id", id);
  fetch(apiHost+"/redo",{
    mode: 'cors',
    method: 'POST',
    credentials: 'include',
    body: params,
  }).then(response => {
    if (response.ok) {
      window.location.reload(false);
    }
  });
}
</script>
</body>
</html>
`

const infoTemplate = `<html>
<head>
  <title>Server information</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/semantic-ui@2.4.2/dist/semantic.min.css">
  <script src="https://code.jquery.com/jquery-3.4.1.min.js" integrity="sha256-CSXorXvZcTkaix6Yvo6HppcZGetbYMGWSFlBw8HfCJo=" crossorigin="anonymous"></script>
  <script src="https://cdn.jsdelivr.net/npm/semantic-ui@2.4.2/dist/semantic.min.js"></script>
  <style>
    i.amber.icon {color: #FFA000;}
  </style>
</head>
<body>

<div class="ui menu inverted huge">
  <div class="header item">
    <a href="/">Dashboard</a>
  </div>

  <div class="item right">
    <a href="/server_info"><i class="info icon"></i>Info</a>
  </div>
</div>

<div class="ui container">
  <table class="ui table definition">
    <tbody>
    <tr>
      <td>Supported Bazel version</td>
      <td>
        <div class="ui bulleted list" style="column-count: 3">
          {{- range .Versions }}
          <div class="item">{{ . }}</div>
          {{- end }}
        </div>
      </td>
    </tr>
    <tr>
      <td>Builder</td>
      <td>Running</td>
    </tr>
    <tr>
      <td>Schema version</td>
      <td></td>
    </tr>
    </tbody>
  </table>
</div>

</body>
</html>
`
