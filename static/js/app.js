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
