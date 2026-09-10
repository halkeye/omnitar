async function main() {
  const features = await fetch("/features").then((resp) => resp.json());
  if (features.installable) {
    document.querySelector(".slack-corner")?.classList.remove("hide");
    const newURL = new URL(features.installable);
    newURL.searchParams.set(
      "redirect_uri",
      window.location.origin + "/slack/auth",
    );
    document
      .querySelector("a.slack-corner")
      ?.setAttribute("href", newURL.toString());
  }

  Array.from(document.querySelectorAll("copy-paste"))
    .filter((elm) => elm.textContent.includes("webcomponent.js"))
    .forEach(
      (elm) =>
        (elm.textContent = elm.textContent.replace(
          /https:\/\/[\w\.]+/,
          window.location.origin,
        )),
    );
}
main().catch(console.error);
