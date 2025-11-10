// HTMX event handlers for navigation
document.body.addEventListener('htmx:beforeRequest', function(evt) {
    console.log('Navigation request started');
});

document.body.addEventListener('htmx:afterRequest', function(evt) {
    console.log('Navigation completed');
});
