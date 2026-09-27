import "./index-source.css";
import "./copy-paste-entry.js";

// Web Awesome styles
import "@awesome.me/webawesome/dist/styles/webawesome.css";
import "@awesome.me/webawesome/dist/styles/utilities.css";
import "@awesome.me/webawesome/dist/styles/native.css";

// Import the components you want to use
import "@awesome.me/webawesome/dist/components/page/page.js";
import "@awesome.me/webawesome/dist/components/button/button.js";
import "@awesome.me/webawesome/dist/components/toast/toast.js";
import "@awesome.me/webawesome/dist/components/toast-item/toast-item.js";
import "@awesome.me/webawesome/dist/components/callout/callout.js";
import "@awesome.me/webawesome/dist/components/icon/icon.js";
import "@awesome.me/webawesome/dist/components/tab/tab.js";
import "@awesome.me/webawesome/dist/components/tab-group/tab-group.js";
import "@awesome.me/webawesome/dist/components/tab-panel/tab-panel.js";

window.docReady = function docReady(fn) {
  if (
    document.readyState === "complete" ||
    document.readyState === "interactive"
  ) {
    setTimeout(fn, 1);
  } else {
    document.addEventListener("DOMContentLoaded", fn);
  }
};

document.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return;
  }

  if (target.dataset.confirm && !confirm(target.dataset.confirm)) {
    event.preventDefault();
    event.stopPropagation();
    return;
  }

  if (target.dataset.method && target.href) {
    event.preventDefault();
    event.stopPropagation();
    fetch(target.href, { method: target.dataset.method })
      .then((response) => {
        if (!response.ok) {
          throw new Error(response.statusText);
        }
      })
      .then(() => location.reload())
      .catch((error) => alert(error));
  }
});
