import Toybox.Lang;
import Toybox.Test;

(:test)
function fixPickChoosesFreshest(logger as Logger) as Boolean {
    var now = 100000;
    var fresh = [43.2, 76.8, now - 100];
    var older = [43.3, 76.9, now - 500];
    var old = [43.4, 77.0, now - 7201];
    Test.assert(Fix.pick(null, null, now) == null);
    Test.assert(Fix.pick(fresh, old, now) == fresh);
    Test.assert(Fix.pick(old, old, now) == null);
    Test.assert(Fix.pick(old, fresh, now) == fresh);
    Test.assert(Fix.pick(older, fresh, now) == fresh);
    Test.assert(Fix.pick(fresh, older, now) == fresh);
    Test.assert(Fix.pick(null, older, now) == older);
    var tieStored = [1.0, 2.0, now - 100];
    Test.assert(Fix.pick(fresh, tieStored, now) == fresh);
    Test.assert(Fix.pick([1.0, 2.0], null, now) == null);
    Test.assert(Fix.pick([null, 2.0, now], null, now) == null);
    return true;
}
