(function () {
  let controller = new AbortController();

  window.docReady = function docReady(fn) {
    // see if DOM is already available
    if (
      document.readyState === "complete" ||
      document.readyState === "interactive"
    ) {
      // call on next available tick
      setTimeout(fn, 1);
    } else {
      document.addEventListener("DOMContentLoaded", fn);
    }
  };

  document.addEventListener("click", function (e) {
    if (!e.target) {
      return;
    }

    const dataset = e.target.dataset;
    if (!dataset) {
      return;
    }

    if (dataset.confirm) {
      if (!confirm(dataset.confirm)) {
        e.preventDefault();
        e.stopPropagation();
        return false;
      }
    }

    if (dataset.method) {
      e.preventDefault();
      e.stopPropagation();
      fetch(e.target.href, { method: dataset.method })
        .then((r) => {
          if (!r.ok) {
            throw new Error(r.statusText);
          }
        })
        .then(() => {
          location.reload();
        })
        .catch((e) => {
          alert(e);
        });
      return;
    }
  });
})();
