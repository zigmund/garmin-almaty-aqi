import Toybox.Lang;

// Category names for the glance, longest form first; the view takes the first that fits.
// Glance-only: these string resources are glance-scoped, not visible to the background.
(:glance)
module AqiLabel {
    function forms(cat as Number) as Array<ResourceId> {
        switch (cat) {
            case 0: return [Rez.Strings.Cat0];
            case 1: return [Rez.Strings.Cat1];
            case 2: return [Rez.Strings.Cat2, Rez.Strings.Cat2Mid, Rez.Strings.Cat2Short];
            case 3: return [Rez.Strings.Cat3];
            case 4: return [Rez.Strings.Cat4];
            default: return [Rez.Strings.Cat5];
        }
    }
}
