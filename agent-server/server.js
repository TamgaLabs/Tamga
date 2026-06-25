const http = require("http");
const { exec } = require("child_process");
const fs = require("fs");
const path = require("path");

const PORT = 9000;
const DATA_DIR = "/data";
const tasks = new Map();

function loadTasks() {
  try {
    const dir = fs.readdirSync(DATA_DIR);
    dir.filter((f) => f.endsWith(".json")).forEach((f) => {
      const d = JSON.parse(fs.readFileSync(path.join(DATA_DIR, f), "utf-8"));
      tasks.set(d.id, d);
    });
  } catch {}
}

function saveTask(task) {
  try {
    if (!fs.existsSync(DATA_DIR)) fs.mkdirSync(DATA_DIR, { recursive: true });
    fs.writeFileSync(path.join(DATA_DIR, `${task.id}.json`), JSON.stringify(task));
  } catch (e) {
    console.error("save task error:", e.message);
  }
}

loadTasks();

function runOpencode(task, projectDir) {
  task.status = "processing";
  saveTask(task);

  const logFile = path.join(DATA_DIR, `${task.id}.log`);
  const cmd = `cd "${projectDir}" && echo "${task.message}" | opencode --stdin --diff 2>&1`;

  exec(cmd, { timeout: 600000, maxBuffer: 10 * 1024 * 1024 }, (err, stdout, stderr) => {
    const log = stdout + (stderr ? "\nSTDERR:\n" + stderr : "");
    try {
      fs.writeFileSync(logFile, log);
    } catch {}

    if (err) {
      task.status = "failed";
      task.response = log.slice(0, 5000);
      task.diff = "";
      task.completed_at = new Date().toISOString();
    } else {
      task.status = "completed";
      const parts = log.split("---DIFF---");
      task.response = parts[0] ? parts[0].trim().slice(0, 5000) : log.slice(0, 5000);
      task.diff = parts[1] ? parts[1].trim() : "";
      if (!task.diff) {
        const diffMatch = log.match(/^diff[\s\S]*?(?=\n\n|\n$|$)/m);
        task.diff = diffMatch ? diffMatch[0] : "";
      }
      task.completed_at = new Date().toISOString();
    }
    saveTask(task);
  });
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);

  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Content-Type");

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  // POST /chat
  if (req.method === "POST" && url.pathname === "/chat") {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => {
      try {
        const { task_id, message, project_dir } = JSON.parse(body);
        if (!task_id || !message) {
          res.writeHead(400);
          res.end(JSON.stringify({ error: "task_id and message required" }));
          return;
        }

        const task = {
          id: task_id,
          message,
          status: "pending",
          response: "",
          diff: "",
          created_at: new Date().toISOString(),
          completed_at: null,
        };
        tasks.set(task_id, task);
        saveTask(task);

        const projectDir = project_dir || "/project";
        runOpencode(task, projectDir);

        res.writeHead(202);
        res.end(JSON.stringify({ task_id }));
      } catch (e) {
        res.writeHead(400);
        res.end(JSON.stringify({ error: e.message }));
      }
    });
    return;
  }

  // GET /tasks/:id
  const taskMatch = url.pathname.match(/^\/tasks\/(.+)$/);
  if (req.method === "GET" && taskMatch) {
    const taskId = taskMatch[1];
    const task = tasks.get(taskId);
    if (!task) {
      res.writeHead(404);
      res.end(JSON.stringify({ error: "task not found" }));
      return;
    }
    res.writeHead(200);
    res.end(JSON.stringify(task));
    return;
  }

  // GET /health
  if (url.pathname === "/health") {
    res.writeHead(200);
    res.end(JSON.stringify({ status: "ok" }));
    return;
  }

  res.writeHead(404);
  res.end("Not found");
});

server.listen(PORT, "0.0.0.0", () => {
  console.log(`Agent server listening on port ${PORT}`);
});
