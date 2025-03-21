const videoElement = `<video id="videotest" width="0" height="0" src="/.within.website/x/cmd/anubis/static/testdata/black.mp4"></video>`;

export const testVideo = async (testarea) => {
  testarea.innerHTML = videoElement;
  return (await new Promise((resolve) => {
    const video = document.getElementById('videotest');
    video.oncanplay = () => {
      testarea.style.display = "none";
      resolve(true);
    };
    video.onerror = (ev) => {
      testarea.style.display = "none";
      resolve(false);
    };
  }));
};