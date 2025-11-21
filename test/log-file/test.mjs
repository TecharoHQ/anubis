import { statSync } from "fs";

async function getPage(path) {
  return fetch(`http://localhost:8923${path}`, {
    headers: {
      'User-Agent': 'CHALLENGE'
    }
  })
    .then(resp => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then(resp => resp.text());
}

async function getFileSize(filePath) {
  try {
    return statSync(filePath).size;
  } catch (error) {
    return 0;
  }
}

(async () => {
  const logFilePath = "./var/anubis.log";

  // Get initial log file size
  const initialSize = await getFileSize(logFilePath);
  console.log(`Initial log file size: ${initialSize} bytes`);

  // Make 35 requests with different paths
  const requests = [];
  for (let i = 0; i < 35; i++) {
    requests.push(`/test${i}`);
  }

  const resultSheet = {};
  let failed = false;

  for (const path of requests) {
    try {
      const resp = await getPage(path);
      resultSheet[path] = {
        success: true,
        line: resp.split("\n")[0],
      };
    } catch (error) {
      resultSheet[path] = {
        success: false,
        error: error.message,
      };
      console.log(`✗ Request to ${path} failed: ${error.message}`);
      failed = true;
    }
  }

  // Check final log file size
  const finalSize = await getFileSize(logFilePath);
  console.log(`Final log file size: ${finalSize} bytes`);
  console.log(`Size increase: ${finalSize - initialSize} bytes`);

  // Verify that log file size increased
  if (finalSize <= initialSize) {
    console.error("ERROR: Log file size did not increase after making requests!");
    failed = true;
  }

  let successCount = 0;
  for (let [k, v] of Object.entries(resultSheet)) {
    if (!v.success) {
      console.error({ path: k, error: v.error });
    } else {
      successCount++;
    }
  }

  console.log(`Successful requests: ${successCount}/${requests.length}`);

  if (failed) {
    console.error("Test failed: Some requests failed or log file size did not increase");
    process.exit(1);
  } else {
    console.log("Test passed: All requests succeeded and log file size increased");
    process.exit(0);
  }
})();