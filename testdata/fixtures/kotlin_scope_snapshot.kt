package com.example

import app.cash.paparazzi.Paparazzi
import org.junit.Rule
import org.junit.Test

class HomeScreenSnapshotTest {

    @get:Rule
    val paparazzi = Paparazzi()

    @Test
    fun `home screen default state`() {
        paparazzi.snapshot {
            HomeScreen()
        }
    }
}
