(() => {
  let installed = false;
  Object.defineProperty(window, 'ZamModelViewer', {
    configurable: true,
    enumerable: true,
    get() { return undefined; },
    set(Ctor) {
      if (installed || typeof Ctor !== 'function') {
        Object.defineProperty(window, 'ZamModelViewer', { configurable: true, writable: true, value: Ctor });
        return;
      }
      installed = true;
      function Viewer() {
        const inst = new Ctor(...arguments);
        window.__whViewer = inst;
        return inst;
      }
      Viewer.prototype = Ctor.prototype;
      Object.setPrototypeOf(Viewer, Ctor);
      for (const key of Object.keys(Ctor)) Viewer[key] = Ctor[key];
      Object.defineProperty(window, 'ZamModelViewer', { configurable: true, writable: true, value: Viewer });
    },
  });
})();